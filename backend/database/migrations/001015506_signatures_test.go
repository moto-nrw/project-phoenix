package migrations

import (
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignaturesMigrationRequiresAReferenceForSignedSubmissions(t *testing.T) {
	t.Parallel()

	require.Contains(t, signaturesUpSQL, "(method = 'simple_electronic_signature') = (signature_id IS NOT NULL)")
	require.Contains(t, signaturesUpSQL, "REFERENCES audit.signatures(tenant_id, id) ON DELETE CASCADE")
}

func TestSignaturesSchoolErasureDeletesSubmissionsBeforeSignatures(t *testing.T) {
	t.Parallel()

	submissions := strings.Index(schoolErasureDeleteSQL, "DELETE FROM users.parent_announcement_declaration_submissions")
	signatures := strings.Index(schoolErasureDeleteSQL, "DELETE FROM audit.signatures")
	requests := strings.Index(schoolErasureDeleteSQL, "DELETE FROM enrollment.requests")
	require.GreaterOrEqual(t, submissions, 0)
	require.GreaterOrEqual(t, signatures, 0)
	require.GreaterOrEqual(t, requests, 0)
	assert.Less(t, submissions, signatures)
	assert.Less(t, signatures, requests)
}

func TestSignatureSchoolErasureForeignKeyFingerprint(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	var fingerprint string
	require.NoError(t, db.NewRaw(`SELECT encode(sha256(convert_to(COALESCE(string_agg(
 edge.child_table || '|' || edge.constraint_name || '|' || edge.parent_table || '|' || edge.action || '|' || edge.child_columns || '|' || edge.parent_columns,
 E'\n' ORDER BY edge.child_table COLLATE "C", edge.constraint_name COLLATE "C"), ''), 'UTF8')), 'hex')
 FROM (
  SELECT cn.nspname || '.' || cc.relname AS child_table,f.conname AS constraint_name,pn.nspname || '.' || pc.relname AS parent_table,
   f.confdeltype::text AS action,string_agg(ca.attname,',' ORDER BY key.ordinality) AS child_columns,string_agg(pa.attname,',' ORDER BY key.ordinality) AS parent_columns
  FROM pg_constraint f JOIN pg_class cc ON cc.oid=f.conrelid JOIN pg_namespace cn ON cn.oid=cc.relnamespace
  JOIN pg_class pc ON pc.oid=f.confrelid JOIN pg_namespace pn ON pn.oid=pc.relnamespace
  CROSS JOIN LATERAL unnest(f.conkey,f.confkey) WITH ORDINALITY AS key(child_col,parent_col,ordinality)
  JOIN pg_attribute ca ON ca.attrelid=cc.oid AND ca.attnum=key.child_col
  JOIN pg_attribute pa ON pa.attrelid=pc.oid AND pa.attnum=key.parent_col
  WHERE f.contype='f' AND pn.nspname || '.' || pc.relname IN
   (SELECT item->>'name' FROM jsonb_array_elements(platform.school_erasure_plan()) AS item)
  GROUP BY f.oid,cn.nspname,cc.relname,f.conname,pn.nspname,pc.relname,f.confdeltype
 ) AS edge`).Scan(t.Context(), &fingerprint))
	// The installed preflight is the newest reinstall of the erasure
	// functions: development's constant, or a later migration's copy of it.
	var installed string
	require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('platform.school_erasure_preflight(bigint,bigint)'::regprocedure)`).Scan(t.Context(), &installed))
	assert.Contains(t, installed, fingerprint)
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
	assert.Contains(t, signaturesSchoolErasurePreflightDownSQL, "4b396b704fd400fca1a02a347ac362b01314ba4ea9fb654aa50cecd2aef28334")
}
