//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// testCfg holds service URLs and credentials loaded from the environment.
type testCfg struct {
	APIURL     string
	AdminEmail string
	AdminPass  string
	adminToken string // populated by TestMain after admin login
}

func loadConfig() testCfg {
	return testCfg{
		APIURL:     envOr("API_URL", "http://localhost:8080"),
		AdminEmail: envOr("ADMIN_EMAIL", "admin@pupitre.local"),
		AdminPass:  envOr("ADMIN_PASSWORD", "admin"),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return def
}

// waitReady polls url until it returns 200 or the timeout elapses.
func waitReady(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:gosec,noctx // health-check polling, no context needed
		if err == nil {
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("service not ready at %s after %s", url, timeout)
}

// doJSON sends a JSON request and returns the response.
// The caller is responsible for closing the response body.
func doJSON(t *testing.T, method, url string, body any, token string) *http.Response {
	t.Helper()

	var r io.Reader

	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}

		r = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, r) //nolint:noctx // e2e tests run with default timeouts
	if err != nil {
		t.Fatalf("build request %s %s: %v", method, url, err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request %s %s: %v", method, url, err)
	}

	return resp
}

// readBody reads and closes the response body, returning it as a string.
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()

	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	return string(b)
}

// decodeBody reads the response body into dst.
func decodeBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// mustStatus fails the test if resp.StatusCode != want.
func mustStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()

	if resp.StatusCode != want {
		body := readBody(t, resp)
		t.Fatalf("expected status %d, got %d: %s", want, resp.StatusCode, body)
	}
}

// uniqueEmail generates a unique email address for test isolation.
func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s-%d@e2e.local", prefix, time.Now().UnixNano())
}
