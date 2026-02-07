package settings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"sync"
)

var (
	// encryptionKey is the AES key for encrypting sensitive settings
	encryptionKey []byte
	keyOnce       sync.Once
	keyErr        error
)

// EncryptionKeyEnvVar is the environment variable for the encryption key
const EncryptionKeyEnvVar = "SETTINGS_ENCRYPTION_KEY"

// initEncryptionKey initializes the encryption key from environment
func initEncryptionKey() {
	keyOnce.Do(func() {
		keyStr := os.Getenv(EncryptionKeyEnvVar)
		if keyStr == "" {
			// Generate a random key for development (not persistent across restarts)
			encryptionKey = make([]byte, 32)
			if _, err := rand.Read(encryptionKey); err != nil {
				keyErr = errors.New("failed to generate encryption key")
			}
			return
		}

		// Decode base64 key
		key, err := base64.StdEncoding.DecodeString(keyStr)
		if err != nil {
			keyErr = errors.New("invalid encryption key format (expected base64)")
			return
		}

		if len(key) != 16 && len(key) != 24 && len(key) != 32 {
			keyErr = errors.New("encryption key must be 16, 24, or 32 bytes")
			return
		}

		encryptionKey = key
	})
}

// getEncryptionKey returns the encryption key, initializing if needed
func getEncryptionKey() ([]byte, error) {
	initEncryptionKey()
	if keyErr != nil {
		return nil, keyErr
	}
	return encryptionKey, nil
}

// Encrypt encrypts a plaintext string using AES-GCM
func Encrypt(plaintext string) (string, error) {
	key, err := getEncryptionKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts an AES-GCM encrypted string
func Decrypt(ciphertext string) (string, error) {
	key, err := getEncryptionKey()
	if err != nil {
		return "", err
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// GenerateEncryptionKey generates a new random encryption key
// Returns the key as a base64-encoded string
func GenerateEncryptionKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// IsEncrypted checks if a value appears to be encrypted
// This is a heuristic check based on base64 encoding
func IsEncrypted(value string) bool {
	if len(value) < 32 {
		return false
	}
	_, err := base64.StdEncoding.DecodeString(value)
	return err == nil
}
