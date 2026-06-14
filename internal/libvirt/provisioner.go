package libvirt

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	golibvirt "github.com/digitalocean/go-libvirt"
	"github.com/rs/zerolog/log"
)

// ProvisionRequest describes a machine to provision from an OS image URL.
type ProvisionRequest struct {
	MachineID          string
	ImageURL           string
	ImageCapacityBytes uint64
}

// Provisioner creates libvirt volumes and starts existing domains for allocated instances.
type Provisioner struct {
	cfg Config
}

// NewProvisioner returns a provisioner for the given libvirt configuration.
func NewProvisioner(cfg Config) *Provisioner {
	if cfg.StoragePool == "" {
		cfg.StoragePool = "default"
	}
	if cfg.DefaultVolumeBytes == 0 {
		cfg.DefaultVolumeBytes = defaultVolumeBytes
	}
	cfg.Endpoint = SanitizeEndpoint(cfg.Endpoint)
	return &Provisioner{cfg: cfg}
}

func (p *Provisioner) Enabled() bool {
	return p != nil && p.cfg.Endpoint != ""
}

// ProvisionMachine downloads an OS image, creates a storage volume, and starts the existing domain.
func (p *Provisioner) ProvisionMachine(ctx context.Context, req ProvisionRequest) error {
	if !p.Enabled() {
		return fmt.Errorf("libvirt provisioner is not configured")
	}

	machineID := canonicalMachineID(req.MachineID)
	if machineID == "" {
		return fmt.Errorf("machine id is required")
	}
	if strings.TrimSpace(req.ImageURL) == "" {
		return fmt.Errorf("image url is required")
	}

	log.Info().
		Str("machine_id", machineID).
		Str("image_url", req.ImageURL).
		Msg("starting libvirt provisioning")

	l, err := Connect(p.cfg.Endpoint)
	if err != nil {
		return err
	}
	defer l.Disconnect()

	domain, err := lookupDomainByMachineID(l, machineID)
	if err != nil {
		return err
	}

	if err := stopDomainIfRunning(l, domain, machineID); err != nil {
		return err
	}

	pool, err := l.StoragePoolLookupByName(p.cfg.StoragePool)
	if err != nil {
		return fmt.Errorf("lookup storage pool %q: %w", p.cfg.StoragePool, err)
	}

	volName := volumeName(machineID)
	if err := deleteVolumeIfExists(l, pool, volName); err != nil {
		return err
	}

	imageFormat := imageFormatFromURL(req.ImageURL)
	imageSize, body, err := openImage(ctx, req.ImageURL)
	if err != nil {
		return err
	}
	defer body.Close()

	volCapacity := volumeCapacity(imageSize, req.ImageCapacityBytes, p.cfg.DefaultVolumeBytes)
	vol, err := createVolume(l, pool, volName, volCapacity, imageFormat)
	if err != nil {
		return err
	}

	if err := l.StorageVolUpload(vol, body, 0, 0, 0); err != nil {
		_ = l.StorageVolDelete(vol, 0)
		return fmt.Errorf("upload image to volume %q: %w", volName, err)
	}

	volPath, err := l.StorageVolGetPath(vol)
	if err != nil {
		return fmt.Errorf("get volume path for %q: %w", volName, err)
	}

	domain, err = updateDomainBootDisk(l, domain, volPath, imageFormat, p.cfg.StoragePool, volName)
	if err != nil {
		return err
	}

	if err := startDomain(l, domain, machineID); err != nil {
		return err
	}

	log.Info().
		Str("machine_id", machineID).
		Str("volume", volName).
		Str("volume_path", volPath).
		Msg("libvirt provisioning complete")

	return nil
}

// ReleaseMachine stops the existing domain and deletes its root volume.
func (p *Provisioner) ReleaseMachine(ctx context.Context, machineID string) error {
	if !p.Enabled() {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	machineID = canonicalMachineID(machineID)
	if machineID == "" {
		return nil
	}

	l, err := Connect(p.cfg.Endpoint)
	if err != nil {
		return err
	}
	defer l.Disconnect()

	if domain, err := lookupDomainByMachineID(l, machineID); err == nil {
		if err := stopDomainIfRunning(l, domain, machineID); err != nil {
			log.Warn().Err(err).Str("machine_id", machineID).Msg("failed to stop libvirt domain")
		}
	}

	pool, err := l.StoragePoolLookupByName(p.cfg.StoragePool)
	if err != nil {
		return fmt.Errorf("lookup storage pool %q: %w", p.cfg.StoragePool, err)
	}

	volName := volumeName(machineID)
	if err := deleteVolumeIfExists(l, pool, volName); err != nil {
		return err
	}

	log.Info().Str("machine_id", machineID).Msg("released libvirt volume")
	return nil
}

func deleteVolumeIfExists(l *golibvirt.Libvirt, pool golibvirt.StoragePool, name string) error {
	vol, err := l.StorageVolLookupByName(pool, name)
	if err != nil {
		return nil
	}
	if err := l.StorageVolDelete(vol, 0); err != nil {
		return fmt.Errorf("delete volume %q: %w", name, err)
	}
	return nil
}

func createVolume(l *golibvirt.Libvirt, pool golibvirt.StoragePool, name string, capacity uint64, format string) (golibvirt.StorageVol, error) {
	xmlDesc, err := volumeXML(name, capacity, format)
	if err != nil {
		return golibvirt.StorageVol{}, err
	}

	vol, err := l.StorageVolCreateXML(pool, xmlDesc, 0)
	if err != nil {
		return golibvirt.StorageVol{}, fmt.Errorf("create volume %q: %w", name, err)
	}
	return vol, nil
}

func openImage(ctx context.Context, imageURL string) (int64, io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("create image download request: %w", err)
	}

	client := &http.Client{Timeout: 2 * time.Hour}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("download image from %q: %w", imageURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return 0, nil, fmt.Errorf("download image from %q: HTTP %s", imageURL, resp.Status)
	}

	return resp.ContentLength, resp.Body, nil
}

func volumeCapacity(imageSize int64, imageCapacityBytes, defaultBytes uint64) uint64 {
	capacity := defaultBytes
	if imageCapacityBytes > capacity {
		capacity = imageCapacityBytes
	}
	if imageSize > 0 && uint64(imageSize) > capacity {
		capacity = uint64(imageSize)
	}
	return capacity
}

func volumeName(machineID string) string {
	return machineID + "-root"
}

func imageFormatFromURL(imageURL string) string {
	ext := strings.ToLower(path.Ext(imageURL))
	switch ext {
	case ".qcow2":
		return "qcow2"
	case ".raw":
		return "raw"
	default:
		return "qcow2"
	}
}

type volumeSpec struct {
	XMLName  xml.Name `xml:"volume"`
	Name     string   `xml:"name"`
	Capacity struct {
		Value uint64 `xml:",chardata"`
		Unit  string `xml:"unit,attr"`
	} `xml:"capacity"`
	Target struct {
		Format struct {
			Type string `xml:"type,attr"`
		} `xml:"format"`
	} `xml:"target"`
}

func volumeXML(name string, capacity uint64, format string) (string, error) {
	spec := volumeSpec{Name: name}
	spec.Capacity.Value = capacity
	spec.Capacity.Unit = "bytes"
	spec.Target.Format.Type = format

	out, err := xml.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("marshal volume xml: %w", err)
	}
	return string(out), nil
}
