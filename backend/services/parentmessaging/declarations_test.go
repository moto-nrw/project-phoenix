package parentmessaging

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var declarationNow = time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)

func consentRules(signers string, revocable bool, deadline *time.Time) DeclarationRules {
	return DeclarationRules{
		Kind: DeclarationKindConsent, Signers: signers, Revocable: revocable, Deadline: deadline,
	}
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
		{"nobody may declare", consentRules("any", false, nil), nil, nil, DeclarationStateNoSigner},
		{"nothing yet", consentRules("any", false, nil), []int64{1, 2}, nil, DeclarationStateOpen},
		{"one of any agreed", consentRules("any", false, nil), []int64{1, 2}, map[int64]string{1: "agreed"}, DeclarationStateAgreed},
		{"one of all agreed", consentRules("all", false, nil), []int64{1, 2}, map[int64]string{1: "agreed"}, DeclarationStatePartial},
		{"all agreed", consentRules("all", false, nil), []int64{1, 2}, map[int64]string{1: "agreed", 2: "agreed"}, DeclarationStateAgreed},
		{"refusal wins over consent", consentRules("any", false, nil), []int64{1, 2}, map[int64]string{1: "agreed", 2: "declined"}, DeclarationStateDeclined},
		{"revocation wins over consent", consentRules("any", true, nil), []int64{1, 2}, map[int64]string{1: "agreed", 2: "revoked"}, DeclarationStateRevoked},
		{"lost permission no longer counts", consentRules("any", false, nil), []int64{2}, map[int64]string{1: "agreed"}, DeclarationStateOpen},
		{"deadline passed unanswered", consentRules("any", false, &past), []int64{1}, nil, DeclarationStateExpired},
		{"deadline passed after consent", consentRules("any", false, &past), []int64{1}, map[int64]string{1: "agreed"}, DeclarationStateAgreed},
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
		{"not permitted", consentRules("any", true, nil), false, "", []string{}},
		{"fresh consent", consentRules("any", false, nil), true, "", []string{"agreed", "declined"}},
		{"correct a refusal", consentRules("any", false, nil), true, "declined", []string{"agreed"}},
		{"correct a consent that cannot be revoked", consentRules("any", false, nil), true, "agreed", []string{"declined"}},
		{"revocable consent is withdrawn by revoking", consentRules("any", true, nil), true, "agreed", []string{"revoked"}},
		{"consent again after revoking", consentRules("any", true, nil), true, "revoked", []string{"agreed", "declined"}},
		{"after the deadline only revocation", consentRules("any", true, &past), true, "agreed", []string{"revoked"}},
		{"after the deadline nothing new", consentRules("any", true, &past), true, "", []string{}},
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
	rules := consentRules("any", false, nil)
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
