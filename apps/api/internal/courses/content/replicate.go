package content

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/jojo91550/pupitre2/apps/api/internal/platform/httpx"
)

// replicatedPrefix is the URL path prefix under which replicated module
// resources are served once downloaded.
const replicatedPrefix = "/uploads/"

// replicateDirPerm is the permission used when creating the uploads
// directory.
const replicateDirPerm = 0o750

// maxReplicateExtLength is the longest file extension (including the
// leading dot) considered valid when deriving a cached filename.
const maxReplicateExtLength = 5

// ReplicatedPath returns the local /uploads/ URL path for a replicated
// module resource.
// For text modules or when replication is false, it returns the original
// Src unchanged. The downloaded file is cached on disk; subsequent calls
// skip the download.
func ReplicatedPath(ctx context.Context, mod Module, uploadsDir string) string {
	if !mod.Replication || mod.Type == ModuleTypeText {
		return mod.Src
	}

	if mod.Src == "" {
		return ""
	}

	ext := extension(mod.Src)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(mod.Src)))[:16]
	filename := hash + ext
	localPath := filepath.Join(uploadsDir, filename)

	_, err := os.Stat(localPath)
	if err == nil {
		return replicatedPrefix + filename
	}

	// Deduplicated by destination: when a hundred learners open the same
	// uncached video at once, one of them downloads it and the rest wait
	// for that download. Without this they all downloaded the same file
	// concurrently into the same path, each truncating the others' writes.
	_, err, _ = replicationGroup.Do(localPath, func() (any, error) {
		return nil, downloadFile(ctx, mod.Src, localPath)
	})
	if err != nil {
		zap.L().Error("replication: download failed — returning local path anyway",
			zap.String("src", mod.Src), zap.String("dest", localPath), zap.Error(err))

		return replicatedPrefix + filename
	}

	zap.L().Info("replication: resource cached", zap.String("src", mod.Src), zap.String("local", filename))

	return replicatedPrefix + filename
}

// replicationGroup collapses concurrent downloads of the same destination
// into one.
//
//nolint:gochecknoglobals // process-wide deduplication of an on-disk cache
var replicationGroup singleflight.Group

// maxReplicateFileBytes caps how large a replicated resource may be, so a
// mis-configured module source cannot fill the uploads volume.
const maxReplicateFileBytes = 512 << 20

// downloadFile downloads the resource at rawURL and writes it to dest,
// creating any missing parent directories.
//
// The body is written to a temporary file in the destination directory and
// renamed into place only once it is complete, so a reader never sees a
// half-written file and an interrupted download leaves no cache entry
// behind that later calls would mistake for a finished one.
func downloadFile(ctx context.Context, rawURL, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	resp, err := httpx.Do(req) //nolint:bodyclose // httpx.Drain closes it
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}

	defer httpx.Drain(resp)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	err = os.MkdirAll(filepath.Dir(dest), replicateDirPerm)
	if err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	return writeAtomically(dest, io.LimitReader(resp.Body, maxReplicateFileBytes))
}

// writeAtomically streams src into a temporary file alongside dest, then
// renames it over dest.
func writeAtomically(dest string, src io.Reader) error {
	tmp, err := os.CreateTemp(filepath.Dir(dest), "."+filepath.Base(dest)+".part-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tmpName := tmp.Name()

	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName) // no-op once the rename below has succeeded
	}()

	_, err = io.Copy(tmp, src)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	err = tmp.Close()
	if err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	err = os.Rename(tmpName, dest)
	if err != nil {
		return fmt.Errorf("publish file: %w", err)
	}

	return nil
}

// extension returns the lowercased file extension (including the leading
// dot) from rawURL, or "" if none is found or it looks implausibly long.
func extension(rawURL string) string {
	idx := strings.LastIndex(rawURL, ".")
	if idx < 0 {
		return ""
	}

	ext := strings.ToLower(rawURL[idx:])

	idx2 := strings.IndexAny(ext, "?#")
	if idx2 >= 0 {
		ext = ext[:idx2]
	}

	if len(ext) <= maxReplicateExtLength {
		return ext
	}

	return ""
}
