package application

import (
	"bytes"
	"errors"
	"testing"

	peopleValues "github.com/moto-ogs/project-phoenix/modules/peopledirectory"
)

// contextCipher binds a value to its context like the bank cipher.
type contextCipher struct{}

func (contextCipher) BankDataRing() {}

func (contextCipher) Seal(plaintext, aad []byte) ([]byte, error) {
	return append(append(append([]byte{}, aad...), 0), plaintext...), nil
}

func (contextCipher) Open(sealed, aad []byte) ([]byte, error) {
	prefix := append(append([]byte{}, aad...), 0)
	if !bytes.HasPrefix(sealed, prefix) {
		return nil, errors.New("wrong context")
	}
	return sealed[len(prefix):], nil
}

// #6332: a guardian's sealed values are bound to the school as well as to the
// guardian and the field. Real rows cannot carry the same guardian ID at two
// schools, so the school binding is checked on the value itself.
func TestGuardianBankValuesOpenOnlyAtTheirSchool(t *testing.T) {
	t.Parallel()

	iban, holder := "DE89370400440532013000", "Synthetic holder"
	data := &peopleValues.GuardianPayment{TenantID: 5, GuardianProfileID: 7}
	if err := sealGuardianPayment(contextCipher{}, data, 5, &iban, &holder); err != nil {
		t.Fatal(err)
	}
	stored := *data
	stored.IBAN, stored.AccountHolder = nil, nil
	if err := openGuardianPayment(contextCipher{}, &stored, true); err != nil || *stored.IBAN != iban || *stored.AccountHolder != holder {
		t.Fatalf("own school: %v", err)
	}
	moved := *data
	moved.IBAN, moved.AccountHolder, moved.TenantID = nil, nil, 6
	if err := openGuardianPayment(contextCipher{}, &moved, true); err == nil {
		t.Fatal("a value moved to another school must not open")
	}
	if data.IBANMasked == nil || *data.IBANMasked != "•••• 3000" {
		t.Fatalf("mask: %v", data.IBANMasked)
	}
}
