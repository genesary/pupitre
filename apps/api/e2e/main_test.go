//go:build e2e

// Package e2e_test contains end-to-end tests for the pupitre platform.
// Run with:
//
//	go test -tags e2e ./...
//
// Env vars:
//
//	API_URL         (default: http://localhost:8080)
//	ADMIN_EMAIL     (default: admin@pupitre.local)
//	ADMIN_PASSWORD  (required — no default)
package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

// global config populated by TestMain, readable by all test files.
var globalCfg testCfg

// TestMain waits for all configured services to be ready, performs the
// admin login to obtain a shared token, then runs the test suite.
func TestMain(m *testing.M) {
	globalCfg = loadConfig()

	if err := awaitServices(); err != nil {
		fmt.Fprintln(os.Stderr, "e2e: service readiness check failed:", err)
		os.Exit(1)
	}

	tok, err := adminLogin(globalCfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e: admin login failed:", err)
		os.Exit(1)
	}

	globalCfg.adminToken = tok

	os.Exit(m.Run())
}

// awaitServices waits up to 30 s for the API health endpoint.
func awaitServices() error {
	const timeout = 30 * time.Second

	if err := waitReady(globalCfg.APIURL+"/health", timeout); err != nil {
		return fmt.Errorf("pupitre-api: %w", err)
	}

	return nil
}

// adminLogin posts credentials to the API and returns the JWT.
func adminLogin(cfg testCfg) (string, error) {
	body := map[string]string{
		"email":    cfg.AdminEmail,
		"password": cfg.AdminPass,
	}

	b, _ := json.Marshal(body)

	resp, err := http.Post( //nolint:gosec,noctx // test helper — URLs come from env, no context needed
		cfg.APIURL+"/api/auth/login",
		"application/json",
		bytes.NewReader(b),
	)
	if err != nil {
		return "", fmt.Errorf("login request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login returned %d", resp.StatusCode)
	}

	var out struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode login response: %w", err)
	}

	if out.Token == "" {
		return "", fmt.Errorf("login response contained no token")
	}

	return out.Token, nil
}
