package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func TestSignatureRequiredFollowsTheFormAndSkipsStaffEntries(t *testing.T) {
	t.Parallel()
	signed := &enrollmentNative.FormSchema{CoreRequirements: enrollmentNative.CoreRequirements{enrollmentNative.CoreRequirementSignature: true}}
	unsigned := &enrollmentNative.FormSchema{CoreRequirements: enrollmentNative.CoreRequirements{}}

	assert.True(t, signatureRequired(signed, enrollmentNative.RequestSourcePublic))
	assert.True(t, signatureRequired(signed, enrollmentNative.RequestSourceLateInvite), "a late invite is still the parent's submission")
	assert.False(t, signatureRequired(signed, enrollmentNative.RequestSourceAdminManual), "staff enter a manual enrollment; no parent signs it")
	assert.False(t, signatureRequired(unsigned, enrollmentNative.RequestSourcePublic))
	assert.False(t, signatureRequired(nil, enrollmentNative.RequestSourcePublic), "a Basis phase has no form")
}

func TestRequireSignatureRefusesAMissingImageWithTheFieldCode(t *testing.T) {
	t.Parallel()
	intake := &Intake{deps: IntakeDependencies{Fingerprint: func([]byte) string { return "" }}}
	for _, signature := range []*enrollmentNative.SubmitSignature{nil, {Method: "drawn", ImagePNG: "  "}} {
		err := intake.requireSignature(true, signature)
		var rejection *enrollmentNative.InvalidInputError
		if assert.ErrorAs(t, err, &rejection) {
			assert.Equal(t, enrollmentNative.CodeSignatureRequired, rejection.Code)
			assert.Equal(t, "signature", rejection.Field)
		}
	}
	assert.NoError(t, intake.requireSignature(false, nil))
	assert.Error(t, intake.requireSignature(true, &enrollmentNative.SubmitSignature{Method: "drawn", ImagePNG: "x"}),
		"a required signature without a bound ledger is a configuration error")
}
