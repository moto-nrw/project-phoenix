package test

import "maps"

const (
	sessionMintTransactionSite       = "modules/identityaccess/internal/application/account_authentication.go:NewAccountAuthentication:deps.Runtime.WithAdminTx#1"
	sessionMintTransactionReason     = "Authentication/session seam mintUnit: the one administrative unit every account-session mint (persistSessionInTransaction) and refresh rotation (refreshSessionInTransaction) opens. It resolves or revokes global credentials and account-school mappings before mint/renewal or across an account's schools. Portal, active school and relationship checks remain mandatory before session claims or child access are issued."
	authLedgerTransactionSite        = "modules/identityaccess/internal/application/account_authentication.go:NewAccountAuthentication:deps.Runtime.WithTenantTx#1"
	operatorSessionTransactionSite   = "modules/identityaccess/internal/application/operator_authentication.go:NewOperatorAuthentication:deps.Runtime.WithAdminTx#1"
	operatorSessionTransactionReason = "Platform-operator session seam sessionUnit: the one administrative unit an operator login (issueTokenPair: refresh session, last-login stamp and audit row) and an operator refresh rotation (RefreshToken) open. Operators have no school, so no school RLS scope can hold it."
	authLedgerTransactionReason      = "Atomic owner command seam ledgerUnit: the school transaction logAuthEvent and auditLogout open for an authentication event outside an ambient transaction. Its prerequisite reads/locks, mutations and commit hooks form one command and must roll back together. It joins a supplied workflow/root unit; SQL adapters below it require that unit and cannot open one."
)

// The single session-mint pipeline opens the transactions of four former
// sites through two seams of AccountAuthentication: mintUnit replaces the
// admin units of persistSessionInTransaction and refreshSessionInTransaction,
// ledgerUnit the school units of logAuthEvent and auditLogout. Both bind the
// runtime's own methods, so the units are unchanged; only these exact reasons
// are admitted, and the four former keys leave the register. The operator
// session seam sessionUnit does the same for the operator: it replaces the
// admin unit of RefreshToken and also holds the login, which wrote outside
// any transaction before.
func withoutReviewedSessionMintTransactions(current map[string]string) map[string]string {
	remaining := maps.Clone(current)
	for site, reason := range map[string]string{
		sessionMintTransactionSite:     sessionMintTransactionReason,
		authLedgerTransactionSite:      authLedgerTransactionReason,
		operatorSessionTransactionSite: operatorSessionTransactionReason,
	} {
		if remaining[site] == reason {
			delete(remaining, site)
		}
	}
	return remaining
}
