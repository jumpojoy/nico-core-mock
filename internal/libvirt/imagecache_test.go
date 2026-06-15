package libvirt

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeImageDigest(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"ABC123", "abc123"},
		{"sha256:ABC123", "abc123"},
		{"  SHA256:deadbeef  ", "deadbeef"},
	}
	for _, tc := range tests {
		if got := normalizeImageDigest(tc.in); got != tc.want {
			t.Fatalf("normalizeImageDigest(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestImageCacheHit(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	digest := "abc123"
	content := []byte("cached-image-data")
	hasher := sha256.New()
	hasher.Write(content)
	digest = hex.EncodeToString(hasher.Sum(nil))

	cachePath := filepath.Join(cacheDir, digest+".img")
	if err := os.WriteFile(cachePath, content, 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	size, reader, ok, err := tryOpenCachedImage(cacheDir, digest, "http://example.com/image.img")
	if err != nil {
		t.Fatalf("tryOpenCachedImage: %v", err)
	}
	if !ok {
		t.Fatal("expected cache hit")
	}
	defer reader.Close()

	if size != int64(len(content)) {
		t.Fatalf("size = %d, want %d", size, len(content))
	}
}

func TestImageCacheDigestMismatch(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	content := []byte("stale-image")
	cachePath := filepath.Join(cacheDir, "expecteddigest.img")
	if err := os.WriteFile(cachePath, content, 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	_, _, ok, err := tryOpenCachedImage(cacheDir, "expecteddigest", "http://example.com/image.img")
	if err != nil {
		t.Fatalf("tryOpenCachedImage: %v", err)
	}
	if ok {
		t.Fatal("expected cache miss after digest mismatch")
	}
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Fatalf("expected stale cache file to be removed, stat err = %v", err)
	}
}
