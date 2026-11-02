package db

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"

	"golang.org/x/crypto/bcrypt"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

const (
	// defaultAdminEmail is the email of the bootstrapped admin account.
	defaultAdminEmail = "admin@pupitre.local"
	// defaultAdminUsername is the username of the bootstrapped admin account.
	defaultAdminUsername = "admin"
	// bcryptCost is the work factor used when hashing a cleartext password.
	bcryptCost = 12
	// randomAdminPasswordBytes is the entropy of the password given to an
	// admin account created without ADMIN_PASSWORD.
	randomAdminPasswordBytes = 32
	// adminPasswordPollInterval is how often WatchAdminPassword rereads the
	// admin password file for changes.
	adminPasswordPollInterval = 30 * time.Second
)

// WatchAdminPassword polls filePath every 30 s and calls SeedAdmin whenever
// the file content changes.  It is designed to work with Kubernetes Secret
// volumes: the kubelet refreshes the mounted files automatically (within ~60 s)
// when the Secret is updated, so no pod restart is needed to rotate the
// admin password.
//
// The goroutine exits cleanly when ctx is cancelled (graceful shutdown).
func WatchAdminPassword(ctx context.Context, users repository.UserRepository, filePath string) {
	ticker := time.NewTicker(adminPasswordPollInterval)
	defer ticker.Stop()

	// Capture the current file content so we only act on real changes.
	last, _ := os.ReadFile(filePath) //nolint:gosec // path is operator-controlled via ADMIN_PASSWORD_FILE, not user input

	zap.L().Info("admin password watcher started", zap.String("file", filePath))

	for {
		select {
		case <-ctx.Done():
			zap.L().Info("admin password watcher stopped")

			return
		case <-ticker.C:
			current, err := os.ReadFile(filePath) //nolint:gosec // path is operator-controlled via ADMIN_PASSWORD_FILE, not user input
			if err != nil {
				zap.L().Warn("admin password file unreadable", zap.String("path", filePath), zap.Error(err))

				continue
			}

			if bytes.Equal(current, last) {
				continue
			}

			last = current
			password := strings.TrimSpace(string(current))

			zap.L().Info("admin password file changed — re-seeding admin account")

			err = SeedAdmin(ctx, users, password)
			if err != nil {
				zap.L().Error("re-seed admin failed", zap.Error(err))
			} else {
				zap.L().Info("admin password updated successfully (no restart required)")
			}
		}
	}
}

// randomPasswordHash returns the bcrypt hash of a random password that is
// discarded at once.
func randomPasswordHash() (string, error) {
	secret := make([]byte, randomAdminPasswordBytes)

	_, err := rand.Read(secret)
	if err != nil {
		return "", fmt.Errorf("generate admin password: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(base64.RawURLEncoding.EncodeToString(secret)), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash admin password: %w", err)
	}

	return string(hash), nil
}

// SeedAdmin ensures the default admin user exists and carries the configured
// password hash.  It is called once at startup, after migrations.
//
// Resolution order for the password:
//  1. If adminPassword starts with "$2" it is treated as a pre-computed bcrypt
//     hash and used directly (allows secrets to store either a plaintext or a
//     pre-hashed value under the same ADMIN_PASSWORD key).
//  2. If adminPassword is a non-empty plaintext string it is bcrypt-hashed
//     (cost 12) before being stored.
//  3. If adminPassword is empty, an admin account created now gets a random
//     password that nobody knows, and a warning says to set ADMIN_PASSWORD.
//     There is no default password any more (docs/specs/190-backend-defects.md
//     B5): the well-known Admin@1234 opened every unconfigured instance.
//
// Behaviour on conflict (admin email already exists):
//   - If a custom password was provided (cases 1 or 2) the hash is always
//     updated so deployers can rotate the credential by restarting the pod.
//   - If no password was provided (case 3) the existing row is left untouched
//     so a previously set password is not overwritten.
func SeedAdmin(ctx context.Context, users repository.UserRepository, adminPassword string) error {
	var hash string

	useDefault := false

	switch {
	case strings.HasPrefix(adminPassword, "$2"):
		// Pre-computed bcrypt hash — use as-is.
		hash = adminPassword

		zap.L().Info("admin seeding: using pre-computed bcrypt hash from ADMIN_PASSWORD")

	case adminPassword != "":
		// Cleartext password — hash it now.
		h, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcryptCost)
		if err != nil {
			return fmt.Errorf("hash admin password: %w", err)
		}

		hash = string(h)

		zap.L().Info("admin seeding: hashed ADMIN_PASSWORD, updating admin account")

	default:
		// No password configured: a random one, never shown, so the account
		// cannot be signed into until ADMIN_PASSWORD is set.
		random, err := randomPasswordHash()
		if err != nil {
			return err
		}

		hash = random
		useDefault = true

		zap.L().Warn("ADMIN_PASSWORD is not set: the admin account keeps its password, " +
			"or gets a random one if it is created now. Set ADMIN_PASSWORD to sign in as admin.")
	}

	if useDefault {
		// Only insert if no admin exists yet; never overwrite a potentially
		// customised password when running with the built-in fallback.
		err := users.CreateAdminIfAbsent(ctx, defaultAdminUsername, defaultAdminEmail, hash)
		if err != nil {
			return fmt.Errorf("insert default admin: %w", err)
		}

		return nil
	}

	// Custom password: upsert so the hash is refreshed on every restart.
	// This lets operators rotate the password by updating the secret and
	// rolling the pod — no manual SQL required.
	err := users.UpsertAdminPassword(ctx, defaultAdminUsername, defaultAdminEmail, hash)
	if err != nil {
		return fmt.Errorf("upsert admin password: %w", err)
	}

	return nil
}
