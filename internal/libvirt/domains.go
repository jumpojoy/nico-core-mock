package libvirt

import (
	"fmt"
	"strings"

	golibvirt "github.com/digitalocean/go-libvirt"
	"github.com/google/uuid"
)

func lookupDomainByMachineID(l *golibvirt.Libvirt, machineID string) (golibvirt.Domain, error) {
	machineID = canonicalMachineID(machineID)
	if machineID == "" {
		return golibvirt.Domain{}, fmt.Errorf("machine id is required")
	}

	if domain, err := l.DomainLookupByName(machineID); err == nil {
		return domain, nil
	}

	flags := golibvirt.ConnectListDomainsActive | golibvirt.ConnectListDomainsInactive
	domains, _, err := l.ConnectListAllDomains(1, flags)
	if err != nil {
		return golibvirt.Domain{}, fmt.Errorf("list libvirt domains: %w", err)
	}

	for _, domain := range domains {
		for _, id := range domainIDs(domain) {
			if id == machineID {
				return domain, nil
			}
		}
	}

	return golibvirt.Domain{}, fmt.Errorf("libvirt domain not found for machine id %q", machineID)
}

func stopDomainIfRunning(l *golibvirt.Libvirt, domain golibvirt.Domain, machineID string) error {
	state, _, err := l.DomainGetState(domain, 0)
	if err != nil {
		return fmt.Errorf("get domain state for %q: %w", machineID, err)
	}
	if golibvirt.DomainState(state) != golibvirt.DomainRunning {
		return nil
	}
	if err := l.DomainDestroy(domain); err != nil {
		return fmt.Errorf("stop libvirt domain %q: %w", machineID, err)
	}
	return nil
}

func startDomain(l *golibvirt.Libvirt, domain golibvirt.Domain, machineID string) error {
	state, _, err := l.DomainGetState(domain, 0)
	if err != nil {
		return fmt.Errorf("get domain state for %q: %w", machineID, err)
	}
	if golibvirt.DomainState(state) == golibvirt.DomainRunning {
		return nil
	}
	if err := l.DomainCreate(domain); err != nil {
		return fmt.Errorf("start libvirt domain %q: %w", machineID, err)
	}
	return nil
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
