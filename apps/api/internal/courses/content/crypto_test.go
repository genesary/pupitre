package content

import (
	"strings"
	"testing"
)

// TestDeriveKey checks key derivation from a passphrase.
func TestDeriveKey(t *testing.T) {
	t.Parallel()

	key := DeriveKey("my-secret")
	if len(key) != 32 {
		t.Errorf("expected 32-byte key, got %d", len(key))
	}

	// Same input should produce same key
	key2 := DeriveKey("my-secret")
	for i, b := range key {
		if b != key2[i] {
			t.Error("DeriveKey not deterministic")

			break
		}
	}

	// Different input should produce different key
	key3 := DeriveKey("other-secret")
	same := true

	for i, b := range key {
		if b != key3[i] {
			same = false

			break
		}
	}

	if same {
		t.Error("DeriveKey returned same key for different inputs")
	}
}

// TestEncryptDecryptRoundTrip checks encrypt then decrypt returns input.
func TestEncryptDecryptRoundTrip(t *testing.T) {
	t.Parallel()

	key := DeriveKey("test-key")
	plaintext := "hello, world!"

	ciphertext, err := EncryptToken(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptToken failed: %v", err)
	}

	if ciphertext == "" {
		t.Error("expected non-empty ciphertext")
	}

	if ciphertext == plaintext {
		t.Error("ciphertext should differ from plaintext")
	}

	decrypted, err := DecryptToken(ciphertext, key)
	if err != nil {
		t.Fatalf("DecryptToken failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

// TestEncryptDecrypt_EmptyString checks round-tripping an empty string.
func TestEncryptDecrypt_EmptyString(t *testing.T) {
	t.Parallel()

	key := DeriveKey("test-key")

	ciphertext, err := EncryptToken("", key)
	if err != nil {
		t.Fatalf("EncryptToken empty: %v", err)
	}

	decrypted, err := DecryptToken(ciphertext, key)
	if err != nil {
		t.Fatalf("DecryptToken empty: %v", err)
	}

	if decrypted != "" {
		t.Errorf("expected empty string, got %q", decrypted)
	}
}

// TestEncryptToken_DifferentEachCall checks ciphertext non-determinism.
func TestEncryptToken_DifferentEachCall(t *testing.T) {
	t.Parallel()

	key := DeriveKey("test-key")
	plaintext := "same plaintext"

	c1, _ := EncryptToken(plaintext, key)
	c2, _ := EncryptToken(plaintext, key)
	// Due to random nonce, ciphertexts should differ
	if c1 == c2 {
		t.Error("expected different ciphertexts for same plaintext (random nonce)")
	}
}

// TestDecryptToken_InvalidHex checks the error for non-hex input.
func TestDecryptToken_InvalidHex(t *testing.T) {
	t.Parallel()

	key := DeriveKey("test-key")

	_, err := DecryptToken("not-hex!!!", key)
	if err == nil {
		t.Error("expected error for invalid hex")
	}
}

// TestDecryptToken_TooShort checks the error for truncated ciphertext.
func TestDecryptToken_TooShort(t *testing.T) {
	t.Parallel()

	key := DeriveKey("test-key")
	// valid hex but too short to contain nonce
	_, err := DecryptToken("aabb", key)
	if err == nil {
		t.Error("expected error for too-short ciphertext")
	}
}

// TestDecryptToken_WrongKey checks decryption fails with the wrong key.
func TestDecryptToken_WrongKey(t *testing.T) {
	t.Parallel()

	key1 := DeriveKey("key1")
	key2 := DeriveKey("key2")

	ciphertext, err := EncryptToken("secret data", key1)
	if err != nil {
		t.Fatal(err)
	}

	_, err = DecryptToken(ciphertext, key2)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}

// TestDecryptToken_CorruptedCiphertext checks tampered data is rejected.
func TestDecryptToken_CorruptedCiphertext(t *testing.T) {
	t.Parallel()

	key := DeriveKey("test-key")

	ciphertext, err := EncryptToken("some data", key)
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt the last few characters
	corrupted := ciphertext[:len(ciphertext)-4] + "0000"

	_, err = DecryptToken(corrupted, key)
	if err == nil {
		t.Error("expected error for corrupted ciphertext")
	}
}

// TestEncryptToken_LongPlaintext checks encrypting a long plaintext.
func TestEncryptToken_LongPlaintext(t *testing.T) {
	t.Parallel()

	key := DeriveKey("test-key")
	plaintext := strings.Repeat("a", 10000)

	ciphertext, err := EncryptToken(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptToken long: %v", err)
	}

	decrypted, err := DecryptToken(ciphertext, key)
	if err != nil {
		t.Fatalf("DecryptToken long: %v", err)
	}

	if decrypted != plaintext {
		t.Error("round-trip failed for long plaintext")
	}
}

// TestEncryptToken_InvalidKeySize checks the error for a bad key size.
func TestEncryptToken_InvalidKeySize(t *testing.T) {
	t.Parallel()

	// AES requires 16, 24, or 32 byte key; bad size should error
	badKey := []byte("short")

	_, err := EncryptToken("data", badKey)
	if err == nil {
		t.Error("expected error for invalid key size")
	}
}
