package libvirt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

func normalizeImageDigest(digest string) string {
	digest = strings.TrimSpace(strings.ToLower(digest))
	digest = strings.TrimPrefix(digest, "sha256:")
	return digest
}

func imageCachePath(cacheDir, digest, imageURL string) string {
	ext := strings.ToLower(path.Ext(imageURL))
	if ext == "" {
		ext = ".img"
	}
	return filepath.Join(cacheDir, normalizeImageDigest(digest)+ext)
}

func fileSHA256Hex(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func openCachedOrDownloadImage(ctx context.Context, imageURL, digest, cacheDir string) (int64, io.ReadCloser, error) {
	normalizedDigest := normalizeImageDigest(digest)
	if cacheDir != "" && normalizedDigest != "" {
		if size, reader, ok, err := tryOpenCachedImage(cacheDir, normalizedDigest, imageURL); err != nil {
			return 0, nil, err
		} else if ok {
			return size, reader, nil
		}
		return downloadImageToCache(ctx, imageURL, normalizedDigest, cacheDir)
	}
	return openImageHTTP(ctx, imageURL)
}

func tryOpenCachedImage(cacheDir, digest, imageURL string) (int64, io.ReadCloser, bool, error) {
	cachePath := imageCachePath(cacheDir, digest, imageURL)
	info, err := os.Stat(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil, false, nil
		}
		return 0, nil, false, fmt.Errorf("stat cached image %q: %w", cachePath, err)
	}

	gotDigest, err := fileSHA256Hex(cachePath)
	if err != nil {
		return 0, nil, false, fmt.Errorf("hash cached image %q: %w", cachePath, err)
	}
	if gotDigest != digest {
		log.Warn().
			Str("cache_path", cachePath).
			Str("expected_digest", digest).
			Str("actual_digest", gotDigest).
			Msg("cached os image digest mismatch, re-downloading")
		if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
			return 0, nil, false, fmt.Errorf("remove stale cached image %q: %w", cachePath, err)
		}
		return 0, nil, false, nil
	}

	file, err := os.Open(cachePath)
	if err != nil {
		return 0, nil, false, fmt.Errorf("open cached image %q: %w", cachePath, err)
	}

	log.Info().
		Str("cache_path", cachePath).
		Str("digest", digest).
		Int64("size", info.Size()).
		Msg("using cached os image")

	return info.Size(), file, true, nil
}

func downloadImageToCache(ctx context.Context, imageURL, digest, cacheDir string) (int64, io.ReadCloser, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return 0, nil, fmt.Errorf("create image cache dir %q: %w", cacheDir, err)
	}

	cachePath := imageCachePath(cacheDir, digest, imageURL)
	tempPath := cachePath + ".part"

	file, err := os.Create(tempPath)
	if err != nil {
		return 0, nil, fmt.Errorf("create temp cache file %q: %w", tempPath, err)
	}

	hasher := sha256.New()
	size, err := downloadImageToWriter(ctx, imageURL, io.MultiWriter(file, hasher))
	closeErr := file.Close()
	if err != nil {
		_ = os.Remove(tempPath)
		return 0, nil, err
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return 0, nil, fmt.Errorf("close temp cache file %q: %w", tempPath, closeErr)
	}

	gotDigest := hex.EncodeToString(hasher.Sum(nil))
	if gotDigest != digest {
		_ = os.Remove(tempPath)
		return 0, nil, fmt.Errorf("downloaded image digest mismatch: got %s, want %s", gotDigest, digest)
	}

	if err := os.Rename(tempPath, cachePath); err != nil {
		_ = os.Remove(tempPath)
		return 0, nil, fmt.Errorf("finalize cached image %q: %w", cachePath, err)
	}

	log.Info().
		Str("cache_path", cachePath).
		Str("digest", digest).
		Int64("size", size).
		Str("url", imageURL).
		Msg("cached os image")

	opened, err := os.Open(cachePath)
	if err != nil {
		return 0, nil, fmt.Errorf("open cached image %q: %w", cachePath, err)
	}
	return size, opened, nil
}

func openImageHTTP(ctx context.Context, imageURL string) (int64, io.ReadCloser, error) {
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

func downloadImageToWriter(ctx context.Context, imageURL string, writer io.Writer) (int64, error) {
	_, body, err := openImageHTTP(ctx, imageURL)
	if err != nil {
		return 0, err
	}
	defer body.Close()

	written, err := io.Copy(writer, body)
	if err != nil {
		return 0, fmt.Errorf("download image from %q: %w", imageURL, err)
	}
	return written, nil
}
