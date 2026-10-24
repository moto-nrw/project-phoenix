package migrations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignaturesMigrationRequiresAReferenceForSignedSubmissions(t *testing.T) {
	t.Parallel()

	require.Contains(t, signaturesUpSQL, "(method = 'simple_electronic_signature') = (signature_id IS NOT NULL)")
}

func TestSignaturesRollbackRestoresSchoolErasureWithoutSignatures(t *testing.T) {
	t.Parallel()

	for _, statement := range []string{
		signaturesSchoolErasurePlanDownSQL,
		signaturesSchoolErasurePreflightDownSQL,
		signaturesSchoolErasureDeleteDownSQL,
		signaturesSchoolErasurePreviewDownSQL,
		signaturesSchoolErasureResidualDownSQL,
	} {
		assert.NotContains(t, statement, "audit.signatures")
	}
	assert.Contains(t, signaturesSchoolErasurePreflightDownSQL, "35013a7fec9d1763a2712e0cb4ebd1084c916458d44d5a484f3b0f096414a683")
}
