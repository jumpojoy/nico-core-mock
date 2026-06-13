package libvirt

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	golibvirt "github.com/digitalocean/go-libvirt"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// PowerChecker filters inventory machines by libvirt domain power state.
type PowerChecker interface {
	Enabled() bool
	IsPoweredOn(machineID string) bool
}

// NoopChecker exposes every machine when libvirt filtering is disabled.
type NoopChecker struct{}

func (NoopChecker) Enabled() bool              { return false }
func (NoopChecker) IsPoweredOn(string) bool    { return true }

// PowerFilter tracks libvirt domains that are powered on, keyed by machine id.
type PowerFilter struct {
	endpoint        string
	refreshInterval time.Duration

	mu        sync.RWMutex
	poweredOn map[string]struct{}
}

// NewPowerFilter connects to libvirt and periodically refreshes powered-on domains.
func NewPowerFilter(ctx context.Context, endpoint string, refreshInterval time.Duration) (*PowerFilter, error) {
	endpoint = sanitizeEndpoint(endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("libvirt endpoint is required")
	}
	if refreshInterval <= 0 {
		refreshInterval = 30 * time.Second
	}

	filter := &PowerFilter{
		endpoint:        endpoint,
		refreshInterval: refreshInterval,
		poweredOn:       make(map[string]struct{}),
	}

	if err := filter.refresh(); err != nil {
		return nil, err
	}

	go filter.run(ctx)
	return filter, nil
}

func (f *PowerFilter) Enabled() bool { return true }

func (f *PowerFilter) IsPoweredOn(machineID string) bool {
	id := canonicalMachineID(machineID)
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, ok := f.poweredOn[id]
	return ok
}

func (f *PowerFilter) run(ctx context.Context) {
	ticker := time.NewTicker(f.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := f.refresh(); err != nil {
				log.Warn().Err(err).Str("endpoint", f.endpoint).Msg("libvirt refresh failed")
			}
		}
	}
}

func (f *PowerFilter) refresh() error {
	poweredOn, err := listPoweredOnDomainIDs(f.endpoint)
	if err != nil {
		return err
	}

	f.mu.Lock()
	f.poweredOn = poweredOn
	f.mu.Unlock()

	log.Debug().
		Str("endpoint", f.endpoint).
		Int("powered_on_domains", len(poweredOn)).
		Msg("refreshed libvirt domain power state")

	return nil
}

func listPoweredOnDomainIDs(endpoint string) (map[string]struct{}, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse libvirt endpoint %q: %w", endpoint, err)
	}

	l, err := golibvirt.ConnectToURI(parsed)
	if err != nil {
		return nil, fmt.Errorf("connect to libvirt %q: %w", endpoint, err)
	}
	defer l.Disconnect()

	flags := golibvirt.ConnectListDomainsActive | golibvirt.ConnectListDomainsInactive
	domains, _, err := l.ConnectListAllDomains(1, flags)
	if err != nil {
		return nil, fmt.Errorf("list libvirt domains: %w", err)
	}

	poweredOn := make(map[string]struct{})
	for _, domain := range domains {
		state, _, err := l.DomainGetState(domain, 0)
		if err != nil {
			continue
		}
		if golibvirt.DomainState(state) != golibvirt.DomainRunning {
			continue
		}

		for _, id := range domainIDs(domain) {
			poweredOn[id] = struct{}{}
		}
	}

	return poweredOn, nil
}

func domainIDs(domain golibvirt.Domain) []string {
	ids := make([]string, 0, 2)
	if name := strings.TrimSpace(domain.Name); name != "" {
		ids = append(ids, canonicalMachineID(name))
	}
	if formatted := formatDomainUUID(domain.UUID); formatted != "" {
		ids = append(ids, canonicalMachineID(formatted))
	}
	return ids
}

func formatDomainUUID(raw [16]byte) string {
	parsed, err := uuid.FromBytes(raw[:])
	if err != nil {
		return ""
	}
	return parsed.String()
}

func canonicalMachineID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func sanitizeEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	return strings.Trim(endpoint, `"'`)
}
