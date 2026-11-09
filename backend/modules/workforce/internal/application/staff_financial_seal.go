package application

import (
	"bytes"
	"errors"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// Staff bank and tax values are sealed with the bank ring (#6332, ADR 0183).
// The context binds a value to its field, school and staff member: a value
// copied into another field, row or school does not open.
const (
	staffIBANField   = "users.staff_financial_data.iban"
	staffTaxIDField  = "users.staff_financial_data.tax_id"
	staffSocialField = "users.staff_financial_data.social_security_number"
)

var (
	errStaffBankDataSeal = errors.New("staff bank data could not be sealed")
	errStaffBankDataOpen = errors.New("staff bank data does not open")
)

func staffBankContext(field string, tenantID, staffID int64) []byte {
	return []byte(field + "|" + strconv.FormatInt(tenantID, 10) + "|" + strconv.FormatInt(staffID, 10))
}

// openStaffFinancial opens the sealed values of data in place. A field
// without a sealed value keeps the plaintext of a row the backfill has not
// sealed yet.
func openStaffFinancial(cipher workforce.BankDataCipher, data *domain.StaffFinancialData) error {
	if data == nil {
		return nil
	}
	for _, field := range []struct {
		name   string
		sealed []byte
		target **string
	}{
		{staffIBANField, data.IBANSealed, &data.IBAN},
		{staffTaxIDField, data.TaxIDSealed, &data.TaxID},
		{staffSocialField, data.SocialSecurityNumberSealed, &data.SocialSecurityNumber},
	} {
		if len(field.sealed) == 0 {
			continue
		}
		plain, err := cipher.Open(field.sealed, staffBankContext(field.name, data.TenantID, data.StaffID))
		if err != nil {
			return errStaffBankDataOpen
		}
		value := string(plain)
		*field.target = &value
	}
	return nil
}

// sealStaffFinancial stores the sealed form of the section on data, bound to
// tenantID, and its IBAN mask.
func sealStaffFinancial(cipher workforce.BankDataCipher, data *domain.StaffFinancialData, tenantID int64, section domain.FinancialSection) error {
	if tenantID <= 0 || data.StaffID <= 0 {
		return errStaffBankDataSeal
	}
	seal := func(field string, value *string) ([]byte, error) {
		if value == nil || *value == "" {
			return nil, nil
		}
		sealed, err := cipher.Seal([]byte(*value), staffBankContext(field, tenantID, data.StaffID))
		if err != nil {
			return nil, errStaffBankDataSeal
		}
		return sealed, nil
	}
	var err error
	if data.IBANSealed, err = seal(staffIBANField, section.IBAN); err != nil {
		return err
	}
	if data.TaxIDSealed, err = seal(staffTaxIDField, section.TaxID); err != nil {
		return err
	}
	if data.SocialSecurityNumberSealed, err = seal(staffSocialField, section.SocialSecurityNumber); err != nil {
		return err
	}
	data.IBAN, data.TaxID, data.SocialSecurityNumber = section.IBAN, section.TaxID, section.SocialSecurityNumber
	data.IBANMasked = domain.MaskIBAN(section.IBAN)
	return nil
}

// rekeyStaffValue authenticates and re-seals a sealed value, or seals the
// plaintext of a row the backfill has not reached. A sealed value wins over
// a leftover plaintext, which is cleared. Current values keep their bytes.
func rekeyStaffValue(cipher workforce.BankDataRekeyCipher, aad, sealed []byte, plain *string) ([]byte, bool, error) {
	if len(sealed) > 0 {
		next, err := cipher.Rewrap(sealed, aad)
		if err != nil {
			return nil, false, err
		}
		return next, plain != nil || !bytes.Equal(next, sealed), nil
	}
	if plain == nil {
		return nil, false, nil
	}
	if *plain == "" {
		return nil, true, nil
	}
	next, err := cipher.Seal([]byte(*plain), aad)
	if err != nil {
		return nil, false, err
	}
	return next, true, nil
}
