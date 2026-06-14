package statestore

import (
	"testing"

	cwssaws "github.com/NVIDIA/infra-controller/rest-api/workflow-schema/schema/site-agent/workflows/v1"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/state.json"

	vni := uint32(42)
	snap := &Snapshot{
		Version: stateVersion,
		Vpcs: []*cwssaws.Vpc{{
			Id:     &cwssaws.VpcId{Value: "00000000-0000-4000-8000-000000000001"},
			Name:   "test-vpc",
			Status: &cwssaws.VpcStatus{Vni: &vni},
		}},
	}

	if err := Save(path, snap); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Vpcs) != 1 {
		t.Fatalf("expected 1 vpc, got %d", len(loaded.Vpcs))
	}
	if loaded.Vpcs[0].GetName() != "test-vpc" {
		t.Fatalf("unexpected vpc name %q", loaded.Vpcs[0].GetName())
	}
}

func TestLoadMissingFile(t *testing.T) {
	snap, err := Load(t.TempDir() + "/missing.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Vpcs) != 0 {
		t.Fatalf("expected empty snapshot, got %d vpcs", len(snap.Vpcs))
	}
}

func TestIsMutatingMethod(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"/forge.Forge/FindVpcIds", false},
		{"/forge.Forge/GetOsImage", false},
		{"/forge.Forge/CreateVpc", true},
		{"/forge.Forge/AllocateInstance", true},
		{"/forge.Forge/ReleaseInstance", true},
	}
	for _, tc := range tests {
		if got := IsMutatingMethod(tc.method); got != tc.want {
			t.Fatalf("IsMutatingMethod(%q) = %v, want %v", tc.method, got, tc.want)
		}
	}
}
