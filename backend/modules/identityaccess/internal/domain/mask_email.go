package domain

import "strings"

// MaskEmail is the backend's one rule for showing an address without
// disclosing it (#2108): the first character plus *** before the @. A local
// part of one or two characters is hidden entirely, because one character of
// two would reveal half the name. A value without a usable local part becomes
// "***" instead of passing through unmasked.
//
// It serves the MFA and passkey hint ("code sent to j***@example.com"), the
// operator ledger, and every log line that names a recipient; the root reaches
// it through compose.MaskEmail.
func MaskEmail(address string) string {
	local, domain, ok := strings.Cut(address, "@")
	if !ok || local == "" {
		return "***"
	}
	localRunes := []rune(local)
	if len(localRunes) <= 2 {
		return "***@" + domain
	}
	return string(localRunes[0]) + "***@" + domain
}
