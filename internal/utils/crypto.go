package utils

import (
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

// EncryptSyncData encrypts data using XChaCha20-Poly1305.
// The result is CipherText + MAC. Nonce is 24 bytes.
func EncryptSyncData(secret, nonce, plaintext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(secret)
	if err != nil {
		return nil, fmt.Errorf("create aead: %w", err)
	}

	// Encrypt appends the MAC to the ciphertext.
	return aead.Seal(nil, nonce, plaintext, nil), nil
}

// DecryptSyncData decrypts data using XChaCha20-Poly1305.
// The data should be CipherText + MAC. Nonce is 24 bytes.
func DecryptSyncData(secret, nonce, data []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(secret)
	if err != nil {
		return nil, fmt.Errorf("create aead: %w", err)
	}

	plaintext, err := aead.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}
