package application

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	declarations "github.com/moto-nrw/project-phoenix/modules/communication"
)

// A signed answer is intact only with its stored, unchanged signature given
// on the version the answer names (#4785).
func TestWithSubmissionSignatureChecksTheLinkToTheVersion(t *testing.T) {
	t.Parallel()
	id := int64(9)
	sub := &StaffDeclarationSubmission{SignatureID: &id, ContentHash: strings.Repeat("a", 64)}
	intact := DeclarationSubmissionView{IntegrityOK: true}

	got := withSubmissionSignature(intact, sub, map[int64]declarations.DeclarationSignature{
		id: {Method: "drawn", DocumentSHA256: strings.Repeat("a", 64), IntegrityOK: true},
	})
	require.NotNil(t, got.Signature)
	assert.True(t, got.IntegrityOK)

	got = withSubmissionSignature(intact, sub, map[int64]declarations.DeclarationSignature{
		id: {Method: "drawn", DocumentSHA256: strings.Repeat("b", 64), IntegrityOK: true},
	})
	assert.False(t, got.IntegrityOK, "signed on another version")
	assert.False(t, got.Signature.IntegrityOK)

	got = withSubmissionSignature(intact, sub, map[int64]declarations.DeclarationSignature{})
	assert.False(t, got.IntegrityOK, "signature missing")

	unsigned := withSubmissionSignature(intact, &StaffDeclarationSubmission{}, nil)
	assert.True(t, unsigned.IntegrityOK)
	assert.Nil(t, unsigned.Signature)
}
