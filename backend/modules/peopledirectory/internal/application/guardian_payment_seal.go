package application

import (
	"errors"
	"strconv"

	peopleValues "github.com/moto-ogs/project-phoenix/modules/peopledirectory"
)

// Guardian bank values are sealed with the bank ring (#6332, ADR 0183). The
// context binds a value to its field, school and guardian: a value copied
// into another field, row or school does not open.
const (
	guardianIBANField   = "users.guardian_financial_data.iban"
	guardianHolderField = "users.guardian_financial_data.account_holder"
)

var (
	errGuardianBankCipherMissing = errors.New("guardian bank data cipher is not wired; refusing to store or open bank data")
	errGuardianBankDataSeal      = errors.New("guardian bank data could not be sealed")
	errGuardianBankDataOpen      = errors.New("guardian bank data does not open")
)

func guardianBankContext(field string, tenantID, guardianProfileID int64) []byte {
	return []byte(field + "|" + strconv.FormatInt(tenantID, 10) + "|" + strconv.FormatInt(guardianProfileID, 10))
}

// sealGuardianValue seals one optional value; nil and empty stay nil.
func sealGuardianValue(cipher peopleValues.BankDataCipher, field string, tenantID, guardianProfileID int64, value *string) ([]byte, error) {
	if value == nil || *value == "" {
		return nil, nil
	}
	if cipher == nil {
		return nil, errGuardianBankCipherMissing
	}
	if tenantID <= 0 || guardianProfileID <= 0 {
		return nil, errGuardianBankDataSeal
	}
	sealed, err := cipher.Seal([]byte(*value), guardianBankContext(field, tenantID, guardianProfileID))
	if err != nil {
		return nil, errGuardianBankDataSeal
	}
	return sealed, nil
}

// openGuardianValue opens one sealed value; a row without one keeps its
// fallback, the plaintext of a row the backfill has not sealed yet.
func openGuardianValue(cipher peopleValues.BankDataCipher, field string, tenantID, guardianProfileID int64, sealed []byte, fallback *string) (*string, error) {
	if len(sealed) == 0 {
		return fallback, nil
	}
	if cipher == nil {
		return nil, errGuardianBankCipherMissing
	}
	plain, err := cipher.Open(sealed, guardianBankContext(field, tenantID, guardianProfileID))
	if err != nil {
		return nil, errGuardianBankDataOpen
	}
	value := string(plain)
	return &value, nil
}

// openGuardianPayment opens the account holder always and the IBAN only when
// revealIBAN is set: the masked reads use the stored mask and never open the
// IBAN.
func openGuardianPayment(cipher peopleValues.BankDataCipher, data *peopleValues.GuardianPayment, revealIBAN bool) error {
	if data == nil {
		return nil
	}
	holder, err := openGuardianValue(cipher, guardianHolderField, data.TenantID, data.GuardianProfileID, data.AccountHolderSealed, data.AccountHolder)
	if err != nil {
		return err
	}
	data.AccountHolder = holder
	if !revealIBAN {
		if len(data.IBANSealed) > 0 {
			data.IBAN = nil
		}
		return nil
	}
	iban, err := openGuardianValue(cipher, guardianIBANField, data.TenantID, data.GuardianProfileID, data.IBANSealed, data.IBAN)
	if err != nil {
		return err
	}
	data.IBAN = iban
	return nil
}

// guardianIBANMask is the stored mask, or for a row the backfill has not
// sealed yet the mask of its plaintext.
func guardianIBANMask(data *peopleValues.GuardianPayment) *string {
	if data == nil {
		return nil
	}
	if data.IBANMasked != nil {
		return data.IBANMasked
	}
	return maskTailPtr(data.IBAN, 4)
}

// guardianHolderMask is the fixed mask of a stored account holder; the
// length does not leak and nothing is opened.
func guardianHolderMask(data *peopleValues.GuardianPayment) *string {
	if data == nil {
		return nil
	}
	if len(data.AccountHolderSealed) > 0 {
		present := "sealed"
		return maskAllPtr(&present)
	}
	return maskAllPtr(data.AccountHolder)
}

// sealGuardianPayment replaces the stored values of data with the sealed
// form of iban and holder, bound to the guardian's school.
func sealGuardianPayment(cipher peopleValues.BankDataCipher, data *peopleValues.GuardianPayment, tenantID int64, iban, holder *string) error {
	ibanSealed, err := sealGuardianValue(cipher, guardianIBANField, tenantID, data.GuardianProfileID, iban)
	if err != nil {
		return err
	}
	holderSealed, err := sealGuardianValue(cipher, guardianHolderField, tenantID, data.GuardianProfileID, holder)
	if err != nil {
		return err
	}
	data.IBAN, data.AccountHolder = iban, holder
	data.IBANSealed, data.AccountHolderSealed = ibanSealed, holderSealed
	data.IBANMasked = maskTailPtr(iban, 4)
	return nil
}
