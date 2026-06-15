package libvirt

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	iso9660 "github.com/kdomanski/iso9660"
)

func TestBuildConfigDriveISO(t *testing.T) {
	userData := "#cloud-config\npackages:\n  - curl\n"
	isoBytes, err := BuildConfigDriveISO(userData, "00000000-0000-4000-8000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if len(isoBytes) == 0 {
		t.Fatal("expected non-empty iso")
	}

	image, err := iso9660.OpenImage(bytes.NewReader(isoBytes))
	if err != nil {
		t.Fatal(err)
	}

	label, err := image.Label()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(label) != configDriveVolumeLabel {
		t.Fatalf("volume label = %q, want %q", label, configDriveVolumeLabel)
	}

	metaReader, err := openISOFile(image, "openstack", "latest", "meta_data.json")
	if err != nil {
		t.Fatal(err)
	}

	var meta map[string]string
	if err := json.NewDecoder(metaReader).Decode(&meta); err != nil {
		t.Fatal(err)
	}
	if meta["uuid"] != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("meta_data uuid = %q", meta["uuid"])
	}

	userReader, err := openISOFile(image, "openstack", "latest", "user_data")
	if err != nil {
		t.Fatal(err)
	}

	got, err := io.ReadAll(userReader)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != strings.TrimSpace(userData) {
		t.Fatalf("user_data = %q, want %q", string(got), userData)
	}

	if _, err := openISOFile(image, "openstack", "latest", "network_data.json"); err == nil {
		t.Fatal("expected network_data.json to be absent")
	}
}

func openISOFile(image *iso9660.Image, path ...string) (io.Reader, error) {
	root, err := image.RootDir()
	if err != nil {
		return nil, err
	}

	current := root
	for _, part := range path[:len(path)-1] {
		current, err = findISOChild(current, part)
		if err != nil {
			return nil, err
		}
	}

	file, err := findISOChild(current, path[len(path)-1])
	if err != nil {
		return nil, err
	}
	return file.Reader(), nil
}

func findISOChild(dir *iso9660.File, name string) (*iso9660.File, error) {
	children, err := dir.GetAllChildren()
	if err != nil {
		return nil, err
	}
	for _, child := range children {
		if strings.EqualFold(child.Name(), name) {
			return child, nil
		}
	}
	return nil, io.EOF
}

func TestBuildConfigDriveISORequiresUserData(t *testing.T) {
	if _, err := BuildConfigDriveISO("   ", "id"); err == nil {
		t.Fatal("expected error for empty user-data")
	}
}

func TestISOVolumeCapacity(t *testing.T) {
	if isoVolumeCapacity(1) != 64*1024 {
		t.Fatalf("small iso capacity = %d", isoVolumeCapacity(1))
	}
	if isoVolumeCapacity(5000) != 64*1024 {
		t.Fatalf("medium iso capacity = %d", isoVolumeCapacity(5000))
	}
}
