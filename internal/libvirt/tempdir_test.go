package libvirt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTempDir(t *testing.T) {
	t.Parallel()

	got := ResolveTempDir("/custom/tmp", "/data/forge-state.json")
	if got != "/custom/tmp" {
		t.Fatalf("ResolveTempDir(explicit) = %q, want /custom/tmp", got)
	}

	got = ResolveTempDir("", "/data/forge-state.json")
	want := filepath.Join("/data", "tmp")
	if got != want {
		t.Fatalf("ResolveTempDir(state file) = %q, want %q", got, want)
	}
}

func TestConfigureWritableTempDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	got, err := ConfigureWritableTempDir(dir, "")
	if err != nil {
		t.Fatalf("ConfigureWritableTempDir: %v", err)
	}
	if got != dir {
		t.Fatalf("got %q, want %q", got, dir)
	}
	if os.Getenv("TMPDIR") != dir {
		t.Fatalf("TMPDIR = %q, want %q", os.Getenv("TMPDIR"), dir)
	}
}
