package libvirt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	golibvirt "github.com/digitalocean/go-libvirt"
	iso9660 "github.com/kdomanski/iso9660"
)

const configDriveVolumeLabel = "config-2"

// BuildConfigDriveISO builds an OpenStack config-2 ISO containing user-data only.
// network_data is intentionally omitted so cloud-init leaves network configuration unchanged.
func BuildConfigDriveISO(userData, instanceUUID, instanceName string) ([]byte, error) {
	userData = strings.TrimSpace(userData)
	if userData == "" {
		return nil, fmt.Errorf("user-data is required")
	}
	instanceUUID = strings.TrimSpace(instanceUUID)
	if instanceUUID == "" {
		return nil, fmt.Errorf("instance uuid is required")
	}

	meta := map[string]string{"uuid": instanceUUID}
	if instanceName = strings.TrimSpace(instanceName); instanceName != "" {
		meta["name"] = instanceName
		meta["hostname"] = instanceName
	}

	metaData, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("marshal meta_data.json: %w", err)
	}

	writer, err := iso9660.NewWriter()
	if err != nil {
		return nil, fmt.Errorf("create iso writer: %w", err)
	}
	defer writer.Cleanup()

	if err := writer.AddFile(bytes.NewReader(metaData), "openstack/latest/meta_data.json"); err != nil {
		return nil, fmt.Errorf("add meta_data.json: %w", err)
	}
	if err := writer.AddFile(strings.NewReader(userData), "openstack/latest/user_data"); err != nil {
		return nil, fmt.Errorf("add user_data: %w", err)
	}

	var buf bytes.Buffer
	if err := writer.WriteTo(&buf, configDriveVolumeLabel); err != nil {
		return nil, fmt.Errorf("write config drive iso: %w", err)
	}

	return buf.Bytes(), nil
}

func configDriveVolumeName(machineID string) string {
	return machineID + "-config"
}

func isoVolumeCapacity(size int) uint64 {
	const sector = 2048
	capacity := ((size + sector - 1) / sector) * sector
	if capacity < 64*1024 {
		return 64 * 1024
	}
	return uint64(capacity)
}

func uploadVolumeData(l *golibvirt.Libvirt, pool golibvirt.StoragePool, name string, capacity uint64, format string, body io.Reader) error {
	if err := deleteVolumeIfExists(l, pool, name); err != nil {
		return err
	}

	vol, err := createVolume(l, pool, name, capacity, format)
	if err != nil {
		return err
	}

	if err := l.StorageVolUpload(vol, body, 0, 0, 0); err != nil {
		_ = l.StorageVolDelete(vol, 0)
		return fmt.Errorf("upload volume %q: %w", name, err)
	}

	return nil
}
