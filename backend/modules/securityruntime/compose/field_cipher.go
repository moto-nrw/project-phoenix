// Package compose builds the field cipher of the health information (#4722).
package compose

import "github.com/moto-nrw/project-phoenix/modules/securityruntime/internal/domain/fieldcipher"

// NewFieldCipher reads the key ring "id:base64key[,…]"; the first key seals.
func NewFieldCipher(keys string) (*fieldcipher.Cipher, error) { return fieldcipher.Parse(keys) }

// NewBankDataCipher requires a bank ring; legacyKeys explicitly enables the
// temporary read-only transition from the previous health ring (#6252).
func NewBankDataCipher(keys, legacyKeys string) (*fieldcipher.BankCipher, error) {
	return fieldcipher.ParseBank(keys, legacyKeys)
}
