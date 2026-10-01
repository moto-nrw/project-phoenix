package parentmessaging

import (
	"errors"
	"time"
)

// Erklärungen (#3430): the rules that decide what a declaration means for one
// child and what a guardian may still do. They are pure functions over the
// facts (settings, who may declare, each signer's latest action on the
// current version, the clock), shared by the staff status, the reminder, the
// parent feed and the write path, so none of them can disagree.

// Declaration vocabulary. The strings are the database values.
const (
	// DeclarationKindConsent is the only kind: Zustimmung or Ablehnung. A
	// plain read confirmation is the Elternbrief's Lesebestätigung.
	DeclarationKindConsent = "consent"

	DeclarationSignersAny = "any"
	DeclarationSignersAll = "all"

	DeclarationActionAgreed   = "agreed"
	DeclarationActionDeclined = "declined"
	DeclarationActionRevoked  = "revoked"

	DeclarationStateOpen     = "open"
	DeclarationStatePartial  = "partial"
	DeclarationStateAgreed   = "agreed"
	DeclarationStateDeclined = "declined"
	DeclarationStateRevoked  = "revoked"
	DeclarationStateNoSigner = "no_signer"
	DeclarationStateExpired  = "expired"
)

// ErrDeclarationInvalid reports declaration settings that do not fit together.
var ErrDeclarationInvalid = errors.New("declaration: invalid settings")

// DeclarationRules are the settings the rules need.
type DeclarationRules struct {
	Kind      string
	Signers   string
	Revocable bool
	Deadline  *time.Time
}

// Closed reports whether the deadline has passed at now.
func (r DeclarationRules) Closed(now time.Time) bool {
	return r.Deadline != nil && !r.Deadline.After(now)
}

// ValidateDeclarationSettings checks one declaration configuration.
func ValidateDeclarationSettings(kind, signers string) error {
	if kind != DeclarationKindConsent {
		return ErrDeclarationInvalid
	}
	if signers != DeclarationSignersAny && signers != DeclarationSignersAll {
		return ErrDeclarationInvalid
	}
	return nil
}

// DeclarationChildState derives the state of one child for the current
// version. eligible are the accounts that may declare for the child now;
// latest maps an account to its latest action on the current version. An
// action by an account that lost the permission no longer counts: a
// declaration is only as good as the authority behind it at evaluation time.
//
// A refusal or a revocation by any eligible guardian wins over consent: when
// two guardians disagree about a trip, the child does not go.
func DeclarationChildState(rules DeclarationRules, eligible []int64, latest map[int64]string, now time.Time) string {
	if len(eligible) == 0 {
		return DeclarationStateNoSigner
	}
	counts := map[string]int{}
	for _, accountID := range eligible {
		counts[latest[accountID]]++
	}
	switch {
	case counts[DeclarationActionDeclined] > 0:
		return DeclarationStateDeclined
	case counts[DeclarationActionRevoked] > 0:
		return DeclarationStateRevoked
	}
	done := counts[DeclarationActionAgreed]
	switch {
	case done > 0 && (rules.Signers != DeclarationSignersAll || done == len(eligible)):
		return DeclarationStateAgreed
	case done > 0:
		return DeclarationStatePartial
	case rules.Closed(now):
		return DeclarationStateExpired
	default:
		return DeclarationStateOpen
	}
}

// DeclarationAllowedActions lists what an eligible guardian may do next given
// their own latest action on the current version. Repeating the latest
// action is not listed: it would change nothing.
func DeclarationAllowedActions(rules DeclarationRules, canSubmit bool, myLatest string, now time.Time) []string {
	if !canSubmit {
		return []string{}
	}
	actions := []string{}
	if !rules.Closed(now) {
		revocableConsent := rules.Revocable && myLatest == DeclarationActionAgreed
		for _, action := range []string{DeclarationActionAgreed, DeclarationActionDeclined} {
			// After a revocation the guardian may consent again, as with the
			// photo consent; a revocation is not a lock-out. A revocable
			// consent is withdrawn by revoking it, not by a second answer, so
			// the two ways of saying no never appear side by side.
			if action != myLatest && (action != DeclarationActionDeclined || !revocableConsent) {
				actions = append(actions, action)
			}
		}
	}
	if rules.Revocable && myLatest == DeclarationActionAgreed {
		actions = append(actions, DeclarationActionRevoked)
	}
	return actions
}

// DeclarationActionAllowed reports whether action is in the allowed set.
func DeclarationActionAllowed(rules DeclarationRules, canSubmit bool, myLatest, action string, now time.Time) bool {
	for _, allowed := range DeclarationAllowedActions(rules, canSubmit, myLatest, now) {
		if allowed == action {
			return true
		}
	}
	return false
}

// DeclarationOwesAction reports whether the guardian still owes something for
// the child: they may declare, nothing of theirs counts yet, and the child is
// not settled already (with "any", another guardian may have done it).
func DeclarationOwesAction(rules DeclarationRules, canSubmit bool, myLatest, childState string, now time.Time) bool {
	if !canSubmit || myLatest != "" || rules.Closed(now) {
		return false
	}
	return childState == DeclarationStateOpen || childState == DeclarationStatePartial
}
