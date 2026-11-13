package enrollmenthttp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	capability "github.com/moto-ogs/project-phoenix/modules/enrollment"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// TestRolloverStoresEmptyFormValuesAsJSONObjects pins the one empty form of a
// request's answers, consent flags and source metadata (#6092): the rollover
// carries a source whose answers are the JSON literal null, and the new
// request and child store an empty object, and an empty consent evidence
// list as an empty array, never null.
func TestRolloverStoresEmptyFormValuesAsJSONObjects(t *testing.T) {
	t.Parallel()

	env := setupRolloverTest(t)
	ctx := testpkg.Ctx(t)

	grade := int16(1)
	source := seedApprovedChild(t, env, env.sourcePhase.ID, "Nora", "Leer", "nora.leer@example.com", "Emil", "Leer", grade)
	_, err := env.db.NewRaw("UPDATE enrollment.requests SET custom_data = 'null'::jsonb WHERE tenant_id = ? AND id = ?", testpkg.Tenant(t), source.RequestID).Exec(ctx)
	require.NoError(t, err)
	_, err = env.db.NewRaw("UPDATE enrollment.request_children SET custom_data = 'null'::jsonb WHERE tenant_id = ? AND id = ?", testpkg.Tenant(t), source.ID).Exec(ctx)
	require.NoError(t, err)

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, capability.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)
	require.Equal(t, 1, result.RolledCount)

	var request struct {
		CustomData          string `bun:"custom_data"`
		ConsentFlags        string `bun:"consent_flags"`
		SourceMetadata      string `bun:"source_metadata"`
		LegalBlocksSnapshot string `bun:"legal_blocks_snapshot"`
	}
	require.NoError(t, env.db.NewRaw(`SELECT jsonb_typeof(custom_data) AS custom_data, jsonb_typeof(consent_flags) AS consent_flags,
		jsonb_typeof(source_metadata) AS source_metadata, jsonb_typeof(legal_blocks_snapshot) AS legal_blocks_snapshot
		FROM enrollment.requests WHERE tenant_id = ? AND phase_id = ?`, testpkg.Tenant(t), result.Phase.ID).Scan(ctx, &request))
	assert.Equal(t, "object", request.CustomData, "rolled request custom_data")
	assert.Equal(t, "object", request.ConsentFlags, "rolled request consent_flags")
	assert.Equal(t, "object", request.SourceMetadata, "rolled request source_metadata")
	assert.Equal(t, "array", request.LegalBlocksSnapshot, "rolled request legal_blocks_snapshot")

	var childCustomData string
	require.NoError(t, env.db.NewRaw(`SELECT jsonb_typeof(custom_data) FROM enrollment.request_children
		WHERE tenant_id = ? AND rollover_source_child_id = ?`, testpkg.Tenant(t), source.ID).Scan(ctx, &childCustomData))
	assert.Equal(t, "object", childCustomData, "rolled child custom_data")
}
