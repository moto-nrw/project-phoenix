package userpasstest

import "github.com/moto-nrw/project-phoenix/modules/securityruntime/internal/domain/fieldcipher"

// FieldCipher seals and opens single database values, each bound to its
// context: the cipher the roots build from HEALTH_DATA_KEYS (#4722, #5472).
// It lives beside the cheap hashing profile because this package is Security
// Runtime's one test-support seam; the shared fixtures seal a child's health
// field with it like the server does.
type FieldCipher interface {
	Seal(plaintext, context []byte) ([]byte, error)
	Open(sealed, context []byte) ([]byte, error)
}

// ParseFieldCipher reads a key ring "id:base64key[,…]"; the first key seals.
func ParseFieldCipher(keys string) (FieldCipher, error) {
	cipher, err := fieldcipher.Parse(keys)
	if err != nil {
		return nil, err
	}
	return cipher, nil
}
