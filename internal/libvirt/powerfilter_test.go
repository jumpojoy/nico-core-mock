package libvirt

import (
	"testing"

	golibvirt "github.com/digitalocean/go-libvirt"
)

func TestDomainIDs(t *testing.T) {
	domain := golibvirt.Domain{
		Name: "00000000-0000-4000-8000-000000000001",
		UUID: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2},
	}

	ids := domainIDs(domain)
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %d", len(ids))
	}
	if ids[0] != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("unexpected name id %q", ids[0])
	}
	if ids[1] != "00000000-0000-0000-0000-000000000002" {
		t.Fatalf("unexpected uuid id %q", ids[1])
	}
}

func TestNoopChecker(t *testing.T) {
	checker := NoopChecker{}
	if checker.Enabled() {
		t.Fatal("noop checker should be disabled")
	}
	if !checker.IsPoweredOn("any-id") {
		t.Fatal("noop checker should allow all machines")
	}
}

func TestPowerFilterIsPoweredOn(t *testing.T) {
	filter := &PowerFilter{
		endpoint:  "qemu+tcp://example/system",
		poweredOn: map[string]struct{}{"00000000-0000-4000-8000-000000000000": {}},
	}

	if !filter.IsPoweredOn("00000000-0000-4000-8000-000000000000") {
		t.Fatal("expected machine to be powered on")
	}
	if filter.IsPoweredOn("00000000-0000-4000-8000-000000000001") {
		t.Fatal("expected machine to be filtered out")
	}
}
