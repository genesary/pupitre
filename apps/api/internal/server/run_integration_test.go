//go:build integration

// run_integration_test.go boots the whole API through Run against a live
// PostgreSQL instance (TEST_DATABASE_URL), checks that it serves, then stops
// it with SIGTERM. It replaces the start-up tests of the three services
// (docs/specs/190-backend-defects.md B19).

package server

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"
)

// freePort returns a TCP port nothing listens on right now.
func freePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find a free port: %v", err)
	}
	defer func() { _ = listener.Close() }()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected address %v", listener.Addr())
	}

	return addr.Port
}

// TestRun_ServesThenShutsDownOnSIGTERM starts Run, waits for /health, sends
// SIGTERM and expects a clean nil return.
func TestRun_ServesThenShutsDownOnSIGTERM(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	port := freePort(t)

	t.Setenv("DATABASE_URL", url)
	t.Setenv("PORT", fmt.Sprint(port))
	t.Setenv("JWT_SECRET", "an-adequately-long-test-jwt-secret-value")
	t.Setenv("ADMIN_PASSWORD", "Run-Test-Admin-1")
	t.Setenv("SEED_DEV_COURSES", "")
	t.Setenv("SEED_MOCK_DATA", "")
	t.Setenv("CONFIG_PATH", "")

	done := make(chan error, 1)

	go func() { done <- Run() }()

	health := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	deadline := time.Now().Add(30 * time.Second)

	for {
		resp, err := http.Get(health) //nolint:gosec,noctx // fixed local test URL
		if err == nil {
			_ = resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				break
			}
		}

		select {
		case runErr := <-done:
			t.Fatalf("Run returned before serving: %v", runErr)
		default:
		}

		if time.Now().After(deadline) {
			t.Fatal("the API did not answer /health within 30 s")
		}

		time.Sleep(100 * time.Millisecond)
	}

	err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	if err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	select {
	case runErr := <-done:
		if runErr != nil {
			t.Fatalf("Run returned %v, want nil after graceful shutdown", runErr)
		}
	case <-time.After(25 * time.Second):
		t.Fatal("Run did not return after SIGTERM")
	}
}
