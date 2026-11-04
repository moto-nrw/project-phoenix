// Package compose builds the field cipher of the health information (#4722).
package compose

import "github.com/moto-nrw/project-phoenix/modules/securityruntime/internal/domain/fieldcipher"

// NewFieldCipher reads the key ring "id:base64key[,…]"; the first key seals.
func NewFieldCipher(keys string) (*fieldcipher.Cipher, error) { return fieldcipher.Parse(keys) }
