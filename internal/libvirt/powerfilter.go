package libvirt

import (
	"fmt"

	"github.com/rs/zerolog/log"
)

// PowerChecker filters inventory machines by libvirt domain presence.
type PowerChecker interface {
	Enabled() bool
	// IsPoweredOn reports whether a libvirt domain exists for the machine id
	// (by domain name or UUID, running or shut off). The name is historical.
	IsPoweredOn(machineID string) bool
}

// NoopChecker exposes every machine when libvirt filtering is disabled.
type NoopChecker struct{}

func (NoopChecker) Enabled() bool           { return false }
func (NoopChecker) IsPoweredOn(string) bool { return true }

// PowerFilter checks libvirt for a domain matching each machine id.
type PowerFilter struct {
	endpoint string
}

// NewPowerFilter validates libvirt connectivity for domain existence checks.
func NewPowerFilter(endpoint string) (*PowerFilter, error) {
	endpoint = SanitizeEndpoint(endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("libvirt endpoint is required")
	}

	l, err := Connect(endpoint)
	if err != nil {
		return nil, err
	}
	defer l.Disconnect()

	return &PowerFilter{endpoint: endpoint}, nil
}

func (f *PowerFilter) Enabled() bool { return true }

func (f *PowerFilter) IsPoweredOn(machineID string) bool {
	l, err := Connect(f.endpoint)
	if err != nil {
		log.Warn().Err(err).Str("endpoint", f.endpoint).Msg("libvirt connect failed during domain check")
		return false
	}
	defer l.Disconnect()

	_, err = lookupDomainByMachineID(l, machineID)
	return err == nil
}
