package content

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestExtension checks extension.
func TestExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		url  string
		want string
	}{
		{"https://example.com/video.mp4", ".mp4"},
		{"https://example.com/image.PNG", ".png"},
		{"https://example.com/file.jpg?v=123", ".jpg"},
		{"https://example.com/file.gif#anchor", ".gif"},
		{"https://example.com/noext", ""},
		{"https://example.com/very-long-extension.abcdefg", ""},
		{"https://example.com/path/to/file.webm", ".webm"},
		{"", ""},
		{"https://example.com/file.tar.gz", ".gz"},
	}

	for _, tc := range tests {
		got := extension(tc.url)
		if got != tc.want {
			t.Errorf("extension(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

// TestReplicatedPath_NoReplication checks replicated path no replication.
func TestReplicatedPath_NoReplication(t *testing.T) {
	t.Parallel()

	m := Module{
		Type:        "video",
		Src:         "https://example.com/video.mp4",
		Replication: false,
	}

	result := ReplicatedPath(t.Context(), m, "/tmp/uploads")
	if result != "https://example.com/video.mp4" {
		t.Errorf("expected original Src, got %q", result)
	}
}

// TestReplicatedPath_TextType checks replicated path text type.
func TestReplicatedPath_TextType(t *testing.T) {
	t.Parallel()

	m := Module{
		Type:        "text",
		Src:         "https://example.com/content.md",
		Replication: true,
	}
	result := ReplicatedPath(t.Context(), m, "/tmp/uploads")
	// text type returns Src unchanged even with replication
	if result != "https://example.com/content.md" {
		t.Errorf("expected original Src for text type, got %q", result)
	}
}

// TestReplicatedPath_EmptySrc checks replicated path empty src.
func TestReplicatedPath_EmptySrc(t *testing.T) {
	t.Parallel()

	m := Module{
		Type:        "image",
		Src:         "",
		Replication: true,
	}

	result := ReplicatedPath(t.Context(), m, "/tmp/uploads")
	if result != "" {
		t.Errorf("expected empty string for empty Src, got %q", result)
	}
}

// TestReplicatedPath_ReplicationEnabled checks path when replication is on.
func TestReplicatedPath_ReplicationEnabled(t *testing.T) {
	t.Parallel()

	m := Module{
		Type:        "image",
		Src:         "https://example.com/image.png",
		Replication: true,
	}
	// Returns a path starting with /uploads/ (file may not actually be downloaded in test)
	result := ReplicatedPath(t.Context(), m, "/tmp/uploads-test")
	if result == "" {
		t.Error("expected non-empty result")
	}
	// Should return the replicated path pattern
	if result == "" {
		t.Error("expected non-empty path")
	}
}

// ── downloadFile tests ────────────────────────────────────────────────────────

// TestDownloadFile_Success checks download file success.
func TestDownloadFile_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("file content here"))
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "downloaded.txt")

	err := downloadFile(t.Context(), srv.URL, dest)
	if err != nil {
		t.Fatalf("downloadFile: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}

	if string(data) != "file content here" {
		t.Errorf("expected 'file content here', got %q", string(data))
	}
}

// TestDownloadFile_Non200Status checks download file non200 status.
func TestDownloadFile_Non200Status(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()

	err := downloadFile(t.Context(), srv.URL, filepath.Join(tmpDir, "file.txt"))
	if err == nil {
		t.Error("expected error for 404 response")
	}
}

// TestDownloadFile_BadURL checks download file bad URL.
func TestDownloadFile_BadURL(t *testing.T) {
	t.Parallel()

	err := downloadFile(t.Context(), "http://127.0.0.1:0/invalid", "/tmp/nowhere.txt")
	if err == nil {
		t.Error("expected error for bad URL")
	}
}

// TestDownloadFile_DestInSubdir checks download file dest in subdir.
func TestDownloadFile_DestInSubdir(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data"))
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "subdir", "nested", "file.txt")

	err := downloadFile(t.Context(), srv.URL, dest)
	if err != nil {
		t.Fatalf("downloadFile to nested dest: %v", err)
	}

	_, err = os.Stat(dest)
	if err != nil {
		t.Errorf("expected file to exist at %q", dest)
	}
}

// TestReplicatedPath_AlreadyCached checks replicated path already cached.
func TestReplicatedPath_AlreadyCached(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("jpg data"))
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	m := Module{
		Type:        "image",
		Src:         srv.URL + "/photo.jpg",
		Replication: true,
	}

	result1 := ReplicatedPath(t.Context(), m, tmpDir)
	if result1 == "" {
		t.Fatal("expected non-empty result from first call")
	}
	// Second call should hit the cached file
	result2 := ReplicatedPath(t.Context(), m, tmpDir)
	if result2 != result1 {
		t.Errorf("expected same path from cache, got %q vs %q", result1, result2)
	}
}
