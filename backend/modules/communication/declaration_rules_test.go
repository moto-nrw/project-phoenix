package communication

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var declarationNow = time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)

func consentRules(signers string, deadline *time.Time) DeclarationRules {
	return DeclarationRules{Kind: DeclarationKindConsent, Signers: signers, Deadline: deadline}
}

func TestDeclarationChildState(t *testing.T) {
	t.Parallel()
	past := declarationNow.Add(-time.Hour)
	cases := []struct {
		name     string
		rules    DeclarationRules
		eligible []int64
		latest   map[int64]string
		want     string
	}{
		{"nobody may declare", consentRules("any", nil), nil, nil, DeclarationStateNoSigner},
		{"nothing yet", consentRules("any", nil), []int64{1, 2}, nil, DeclarationStateOpen},
		{"one of any agreed", consentRules("any", nil), []int64{1, 2}, map[int64]string{1: "agreed"}, DeclarationStateAgreed},
		{"one of all agreed", consentRules("all", nil), []int64{1, 2}, map[int64]string{1: "agreed"}, DeclarationStatePartial},
		{"all agreed", consentRules("all", nil), []int64{1, 2}, map[int64]string{1: "agreed", 2: "agreed"}, DeclarationStateAgreed},
		{"refusal wins over consent", consentRules("any", nil), []int64{1, 2}, map[int64]string{1: "agreed", 2: "declined"}, DeclarationStateDeclined},
		{"revocation wins over consent", consentRules("any", nil), []int64{1, 2}, map[int64]string{1: "agreed", 2: "revoked"}, DeclarationStateRevoked},
		{"lost permission no longer counts", consentRules("any", nil), []int64{2}, map[int64]string{1: "agreed"}, DeclarationStateOpen},
		{"deadline passed unanswered", consentRules("any", &past), []int64{1}, nil, DeclarationStateExpired},
		{"deadline passed after consent", consentRules("any", &past), []int64{1}, map[int64]string{1: "agreed"}, DeclarationStateAgreed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, DeclarationChildState(tc.rules, tc.eligible, tc.latest, declarationNow))
		})
	}
}

func TestDeclarationAllowedActions(t *testing.T) {
	t.Parallel()
	past := declarationNow.Add(-time.Hour)
	cases := []struct {
		name      string
		rules     DeclarationRules
		canSubmit bool
		mine      string
		want      []string
	}{
		{"not permitted", consentRules("any", nil), false, "", []string{}},
		{"fresh consent", consentRules("any", nil), true, "", []string{"agreed", "declined"}},
		{"correct a refusal", consentRules("any", nil), true, "declined", []string{"agreed"}},
		{"a consent is withdrawn by revoking, not by a refusal", consentRules("any", nil), true, "agreed", []string{"revoked"}},
		{"consent again after revoking", consentRules("any", nil), true, "revoked", []string{"agreed", "declined"}},
		{"after the deadline only revocation", consentRules("any", &past), true, "agreed", []string{"revoked"}},
		{"after the deadline nothing new", consentRules("any", &past), true, "", []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, DeclarationAllowedActions(tc.rules, tc.canSubmit, tc.mine, declarationNow))
		})
	}
}

func TestDeclarationOwesAction(t *testing.T) {
	t.Parallel()
	rules := consentRules("any", nil)
	assert.True(t, DeclarationOwesAction(rules, true, "", DeclarationStateOpen, declarationNow))
	assert.True(t, DeclarationOwesAction(rules, true, "", DeclarationStatePartial, declarationNow))
	assert.False(t, DeclarationOwesAction(rules, true, "agreed", DeclarationStatePartial, declarationNow),
		"a guardian who declared owes nothing more")
	assert.False(t, DeclarationOwesAction(rules, true, "", DeclarationStateAgreed, declarationNow),
		"another guardian settled the child under 'any'")
	assert.False(t, DeclarationOwesAction(rules, false, "", DeclarationStateOpen, declarationNow),
		"pickup-only guardians are never asked")
}

func TestValidateDeclarationSettings(t *testing.T) {
	t.Parallel()
	assert.NoError(t, ValidateDeclarationSettings("consent", "all"))
	assert.NoError(t, ValidateDeclarationSettings("consent", "any"))
	// A read confirmation is the Elternbrief's Lesebestätigung, not a kind.
	assert.ErrorIs(t, ValidateDeclarationSettings("acknowledgement", "any"), ErrDeclarationInvalid)
	assert.ErrorIs(t, ValidateDeclarationSettings("signature", "any"), ErrDeclarationInvalid)
	assert.ErrorIs(t, ValidateDeclarationSettings("consent", "two"), ErrDeclarationInvalid)
}

// TestEveryConsentIsRevocable pins Art. 7 Abs. 3 DSGVO (#6303): a given
// consent can be withdrawn in every state of the declaration, open or past
// its deadline; no school setting takes that away.
func TestEveryConsentIsRevocable(t *testing.T) {
	t.Parallel()
	past := declarationNow.Add(-time.Hour)
	for name, rules := range map[string]DeclarationRules{
		"open":           {Kind: DeclarationKindConsent, Signers: DeclarationSignersAny},
		"deadline later": {Kind: DeclarationKindConsent, Signers: DeclarationSignersAll, Deadline: &[]time.Time{declarationNow.Add(time.Hour)}[0]},
		"deadline past":  {Kind: DeclarationKindConsent, Signers: DeclarationSignersAny, Deadline: &past},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, []string{DeclarationActionRevoked}, DeclarationAllowedActions(rules, true, DeclarationActionAgreed, declarationNow))
			assert.True(t, DeclarationActionAllowed(rules, true, DeclarationActionAgreed, DeclarationActionRevoked, declarationNow))
		})
	}
}
