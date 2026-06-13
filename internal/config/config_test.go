package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadExampleConfig(t *testing.T) {
	path := filepath.Join("..", "..", "config", "machines.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Skip("example config not found")
	}

	inv, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(inv.Machines) != 2 {
		t.Fatalf("expected 2 machines, got %d", len(inv.Machines))
	}
	if len(inv.ExpectedMachines) != 2 {
		t.Fatalf("expected 2 expected machines, got %d", len(inv.ExpectedMachines))
	}
}
