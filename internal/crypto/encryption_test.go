package crypto

import (
	"testing"
)

func TestEncryptor_EncryptDecrypt(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		plaintext string
	}{
		{
			name:      "16 byte key",
			key:       "1234567890123456",
			plaintext: "my-secret-password",
		},
		{
			name:      "24 byte key",
			key:       "123456789012345678901234",
			plaintext: "another-secret-value",
		},
		{
			name:      "32 byte key",
			key:       "12345678901234567890123456789012",
			plaintext: "yet-another-secret",
		},
		{
			name:      "empty plaintext",
			key:       "1234567890123456",
			plaintext: "",
		},
		{
			name:      "special characters",
			key:       "1234567890123456",
			plaintext: "!@#$%^&*()_+-=[]{}|;':\",./<>?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc, err := NewEncryptor(tt.key)
			if err != nil {
				t.Fatalf("NewEncryptor() error = %v", err)
			}

			ciphertext, err := enc.Encrypt(tt.plaintext)
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			if tt.plaintext != "" && ciphertext == tt.plaintext {
				t.Error("Encrypt() ciphertext should not equal plaintext")
			}

			decrypted, err := enc.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			if decrypted != tt.plaintext {
				t.Errorf("Decrypt() = %v, want %v", decrypted, tt.plaintext)
			}
		})
	}
}

func TestNewEncryptor_InvalidKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{
			name: "too short",
			key:  "short",
		},
		{
			name: "17 bytes",
			key:  "12345678901234567",
		},
		{
			name: "empty",
			key:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewEncryptor(tt.key)
			if err != ErrInvalidKey {
				t.Errorf("NewEncryptor() error = %v, want %v", err, ErrInvalidKey)
			}
		})
	}
}

func TestEncryptor_DecryptInvalidCiphertext(t *testing.T) {
	enc, err := NewEncryptor("1234567890123456")
	if err != nil {
		t.Fatalf("NewEncryptor() error = %v", err)
	}

	tests := []struct {
		name        string
		ciphertext  string
		expectError bool
	}{
		{
			name:        "invalid base64",
			ciphertext:  "not-valid-base64!!!",
			expectError: true,
		},
		{
			name:        "valid base64 but invalid ciphertext",
			ciphertext:  "YWJjZGVmZ2g=",
			expectError: true,
		},
		{
			name:        "empty string",
			ciphertext:  "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := enc.Decrypt(tt.ciphertext)
			if tt.expectError && err == nil {
				t.Error("Decrypt() expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Decrypt() unexpected error = %v", err)
			}
		})
	}
}

func TestEncryptor_DifferentEncryptions(t *testing.T) {
	enc, err := NewEncryptor("12345678901234567890123456789012")
	if err != nil {
		t.Fatalf("NewEncryptor() error = %v", err)
	}

	plaintext := "same-plaintext"

	cipher1, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	cipher2, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	if cipher1 == cipher2 {
		t.Error("Two encryptions of the same plaintext should produce different ciphertexts (due to random nonce)")
	}

	dec1, err := enc.Decrypt(cipher1)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	dec2, err := enc.Decrypt(cipher2)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if dec1 != dec2 {
		t.Error("Both decryptions should produce the same plaintext")
	}
}
