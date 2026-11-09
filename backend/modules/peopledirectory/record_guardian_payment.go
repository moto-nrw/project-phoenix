package peopledirectory

import "errors"

func (f *GuardianPayment) Validate() error {
	if f.GuardianProfileID <= 0 {
		return errors.New("guardian_profile_id is required")
	}
	return nil
}
func (f *GuardianPayment) HasData() bool {
	return len(f.IBANSealed) > 0 || len(f.AccountHolderSealed) > 0 ||
		(f.IBAN != nil && *f.IBAN != "") || (f.AccountHolder != nil && *f.AccountHolder != "")
}
