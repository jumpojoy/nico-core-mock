package server

import (
	"testing"

	cwssaws "github.com/NVIDIA/infra-controller/rest-api/workflow-schema/schema/site-agent/workflows/v1"
)

func TestResolveUserDataPrefersInstanceOverride(t *testing.T) {
	t.Parallel()

	srv := &NICoServerImpl{
		oss: map[string]*cwssaws.OperatingSystem{
			"os-1": {
				Id:       &cwssaws.OperatingSystemId{Value: "os-1"},
				UserData: strPtr("#cloud-config\nfrom: os"),
			},
		},
	}

	config := &cwssaws.InstanceConfig{
		Os: &cwssaws.InstanceOperatingSystemConfig{
			Variant: &cwssaws.InstanceOperatingSystemConfig_OsImageId{
				OsImageId: &cwssaws.UUID{Value: "os-1"},
			},
			UserData: strPtr("#cloud-config\nfrom: instance"),
		},
	}

	got := srv.resolveUserData(config)
	want := "#cloud-config\nfrom: instance"
	if got != want {
		t.Fatalf("resolveUserData() = %q, want %q", got, want)
	}
}

func TestResolveUserDataFallsBackToOperatingSystem(t *testing.T) {
	t.Parallel()

	srv := &NICoServerImpl{
		oss: map[string]*cwssaws.OperatingSystem{
			"os-1": {
				Id:       &cwssaws.OperatingSystemId{Value: "os-1"},
				UserData: strPtr("#cloud-config\nfrom: os"),
			},
		},
	}

	config := &cwssaws.InstanceConfig{
		Os: &cwssaws.InstanceOperatingSystemConfig{
			Variant: &cwssaws.InstanceOperatingSystemConfig_OperatingSystemId{
				OperatingSystemId: &cwssaws.OperatingSystemId{Value: "os-1"},
			},
		},
	}

	got := srv.resolveUserData(config)
	want := "#cloud-config\nfrom: os"
	if got != want {
		t.Fatalf("resolveUserData() = %q, want %q", got, want)
	}
}

func strPtr(s string) *string {
	return &s
}
