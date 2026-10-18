package peopledirectory

import "context"

// GuardianPhoneReads retains the caller's transaction, query ordering and
// canonical phone validation, including partial normalization on errors.
type GuardianPhoneReads interface {
	GuardianPhonesForProfile(context.Context, int64) ([]*GuardianProfilePhoneSnapshot, error)
	GuardianPhonesForProfiles(context.Context, []int64) (map[int64][]*GuardianProfilePhoneSnapshot, error)
	NormalizeGuardianPhone(*GuardianProfilePhoneSnapshot) error
	GuardianPhoneTypeOrMobile(string) string
}
