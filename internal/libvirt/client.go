package libvirt

import (
	"fmt"
	"net/url"
	"strings"

	golibvirt "github.com/digitalocean/go-libvirt"
)

// Config holds libvirt connection and provisioning defaults.
type Config struct {
	Endpoint           string
	StoragePool        string
	DefaultVolumeBytes uint64
	ImageCacheDir      string
}

const defaultVolumeBytes = 20 << 30 // 20 GiB

// Connect opens a libvirt connection for the given endpoint URI.
func Connect(endpoint string) (*golibvirt.Libvirt, error) {
	endpoint = SanitizeEndpoint(endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("libvirt endpoint is required")
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse libvirt endpoint %q: %w", endpoint, err)
	}

	l, err := golibvirt.ConnectToURI(parsed)
	if err != nil {
		return nil, fmt.Errorf("connect to libvirt %q: %w", endpoint, err)
	}
	return l, nil
}

// SanitizeEndpoint strips whitespace and stray quotes from a libvirt URI.
func SanitizeEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	return strings.Trim(endpoint, `"'`)
}
