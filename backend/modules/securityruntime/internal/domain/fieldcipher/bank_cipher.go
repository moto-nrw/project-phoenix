package fieldcipher

import "strings"

// BankCipher always seals with the bank ring. Legacy keys are an explicit,
// temporary read-only import, never a substitute for the required bank ring.
type BankCipher struct {
	bank   *Cipher
	legacy *Cipher
}

func ParseBank(keys, legacyKeys string) (*BankCipher, error) {
	bank, err := Parse(keys)
	if err != nil {
		return nil, err
	}
	result := &BankCipher{bank: bank}
	if strings.TrimSpace(legacyKeys) != "" {
		result.legacy, err = Parse(legacyKeys)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (c *BankCipher) Seal(plain, aad []byte) ([]byte, error) { return c.bank.Seal(plain, aad) }

func (c *BankCipher) Open(sealed, aad []byte) ([]byte, error) {
	plain, err := c.bank.Open(sealed, aad)
	if err == nil {
		return plain, nil
	}
	if c.legacy != nil {
		return c.legacy.Open(sealed, aad)
	}
	return nil, ErrOpen
}

// Rewrap authenticates even values already on the active key. It preserves
// their bytes, making repeated passes idempotent; old keys get a fresh nonce.
func (c *BankCipher) Rewrap(sealed, aad []byte) ([]byte, error) {
	_, err := c.bank.Open(sealed, aad)
	id, _, _ := splitSealedField(sealed)
	if err == nil && id == c.bank.activeID {
		return sealed, nil
	}
	plain, err := c.Open(sealed, aad)
	if err != nil {
		return nil, ErrOpen
	}
	return c.bank.Seal(plain, aad)
}
