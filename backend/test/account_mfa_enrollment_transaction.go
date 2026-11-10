package test

import "maps"

const (
	accountMFAEnrollmentTransactionSite   = "modules/identityaccess/internal/application/account_mfa_flow.go:*AccountMFAFlows.Enroll:f.runtime.WithAdminTx#1"
	accountMFAEnrollmentTransactionReason = "Identity credential command Enroll locks the account row a password write and an MFA reset take, compares the redeemed code's credential generation and records the e-mail factor in one administrative unit, so a credential write that committed since the code was redeemed leaves no enrollment (#6518). It replaces the two separate units its existence check and insert opened before; no school or profile row is locked."
)

// #6518 binds the account enrollment to the credential generation: check
// and write need one unit under the account lock. Only this exact site and
// reason are admitted; any other new or reworded site stays rejected.
func withoutReviewedAccountMFAEnrollmentTransaction(current map[string]string) map[string]string {
	if current[accountMFAEnrollmentTransactionSite] != accountMFAEnrollmentTransactionReason {
		return current
	}
	remaining := maps.Clone(current)
	delete(remaining, accountMFAEnrollmentTransactionSite)
	return remaining
}
