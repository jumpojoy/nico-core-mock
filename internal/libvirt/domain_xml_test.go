package libvirt

import (
	"strings"
	"testing"
)

func TestPatchDomainBootDiskXML(t *testing.T) {
	const domainXML = `<domain type='kvm'>
  <name>00000000-0000-4000-8000-000000000000</name>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='raw'/>
      <source file='/var/lib/libvirt/images/old.qcow2'/>
      <target dev='vda' bus='virtio'/>
    </disk>
    <disk type='file' device='cdrom'>
      <target dev='hda' bus='ide'/>
    </disk>
  </devices>
</domain>`

	updated, err := patchDomainBootDiskXML(
		domainXML,
		"/var/lib/libvirt/images/new.qcow2",
		"qcow2",
		"default",
		"00000000-0000-4000-8000-000000000000-root",
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		`type='volume'`,
		`type='qcow2'`,
		`pool='default'`,
		`volume='00000000-0000-4000-8000-000000000000-root'`,
		`device='cdrom'`,
	} {
		if !strings.Contains(updated, want) {
			t.Fatalf("patched xml missing %q:\n%s", want, updated)
		}
	}
	if strings.Contains(updated, "old.qcow2") {
		t.Fatalf("patched xml still references old disk:\n%s", updated)
	}
}

func TestPatchDomainBootDiskXMLPrefersBootOrder(t *testing.T) {
	const domainXML = `<domain type='kvm'>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='raw'/>
      <source file='/old-secondary'/>
      <boot order='2'/>
    </disk>
    <disk type='file' device='disk'>
      <driver name='qemu' type='raw'/>
      <source file='/old-primary'/>
      <boot order='1'/>
    </disk>
  </devices>
</domain>`

	updated, err := patchDomainBootDiskXML(domainXML, "/new", "qcow2", "default", "machine-root")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(updated, "old-primary") {
		t.Fatalf("expected primary boot disk to be updated:\n%s", updated)
	}
	if !strings.Contains(updated, "old-secondary") {
		t.Fatalf("expected secondary disk to remain unchanged:\n%s", updated)
	}
}

func TestPatchDomainBootDiskXMLErrorsWithoutDisk(t *testing.T) {
	_, err := patchDomainBootDiskXML(`<domain><devices></devices></domain>`, "/new", "qcow2", "default", "vol")
	if err == nil {
		t.Fatal("expected error when domain has no disks")
	}
}
