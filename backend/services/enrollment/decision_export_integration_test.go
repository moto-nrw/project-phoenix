package enrollment_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Integration coverage for the ExportPhase coupling: loading the phase
// PII and recording the GDPR audit row are one inseparable operation.
// These hit the real test DB for the data-loading half (via the
// decision env's repos) while keeping the audit repo a controllable
// stub, so a forced audit failure can prove the "no trail, no
// disclosure" invariant end-to-end — the unit tests in
// decision_export_audit_test.go only exercise RecordPhaseExportAudit in
// isolation.

// newExportDecisionService wires a DecisionService with the real
// data-loading repos from the env plus the supplied audit repo, so a
// test can swap in a failing stub. Mirrors the export half of
// setupDecisionTest's config (which omits the audit repo).
func newExportDecisionService(env *decisionTestEnv, auditRepo auditModels.DataAccessLogRepository) enrollmentService.DecisionService {
	return enrollmentService.NewDecisionService(enrollmentService.DecisionServiceConfig{
		RequestRepo:              env.repos.Request,
		RequestChildRepo:         env.repos.RequestChild,
		RequestChildOfferingRepo: env.repos.RequestChildOffering,
		CareOfferingRepo:         env.repos.CareOffering,
		PhaseRepo:                env.repos.Phase,
		FormSchemaRepo:           env.repos.FormSchema,
		DataAccessLogRepo:        auditRepo,
		Logger:                   slog.Default(),
	})
}

// failingSchemaRepo wraps a real FormSchemaRepository but makes FindByID
// fail, simulating a transient read error or a corrupt/orphaned
// request.schema_id. Every other method delegates to the embedded repo.
type failingSchemaRepo struct {
	enrollmentModels.FormSchemaRepository
}

func (failingSchemaRepo) FindByID(_ context.Context, _ int64) (*enrollmentModels.FormSchema, error) {
	return nil, errors.New("schema read failed")
}

// newExportDecisionServiceFailingSchema mirrors newExportDecisionService
// but swaps in a FormSchema repo whose FindByID always errors, so a test
// can prove the export fails closed when a pinned schema cannot be loaded.
func newExportDecisionServiceFailingSchema(env *decisionTestEnv, auditRepo auditModels.DataAccessLogRepository) enrollmentService.DecisionService {
	return enrollmentService.NewDecisionService(enrollmentService.DecisionServiceConfig{
		RequestRepo:              env.repos.Request,
		RequestChildRepo:         env.repos.RequestChild,
		RequestChildOfferingRepo: env.repos.RequestChildOffering,
		CareOfferingRepo:         env.repos.CareOffering,
		PhaseRepo:                env.repos.Phase,
		FormSchemaRepo:           failingSchemaRepo{env.repos.FormSchema},
		DataAccessLogRepo:        auditRepo,
		Logger:                   slog.Default(),
	})
}

// A pinned schema that cannot be loaded must fail the whole export, not
// silently drop that request's custom_data while still writing an audit
// row. The renderer only emits answers for fields found in the loaded
// schemas, so a missing schema would produce an incomplete-but-trusted
// disclosure. There is no legitimate "schema deleted" state either:
// DeleteSchema refuses to drop a schema any request still references.
func TestDecisionService_ExportPhase_SchemaLoadFailureBlocksDisclosure(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	// The submitted request pins the phase's active schema, so the export
	// loop will attempt to load it — and the failing repo makes that error.
	submitOneChild(t, env, "export-schemafail@example.com", "Nora", "SchemaFail")

	audit := &stubAccessLogRepo{}
	svc := newExportDecisionServiceFailingSchema(env, audit)

	data, err := svc.ExportPhase(ctx, env.sourcePhase.ID, int64(4242), "admin", "pdf", "")
	require.Error(t, err, "an unloadable pinned schema must fail the whole export")
	assert.Nil(t, data, "no payload may be returned when a schema could not be loaded")
	assert.Empty(t, audit.entries, "no audit row may be written for a disclosure that never happened")
}

func TestDecisionService_ExportPhase_LoadsDataAndRecordsAudit(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	submitOneChild(t, env, "export-happy@example.com", "Lina", "Export")

	audit := &stubAccessLogRepo{}
	svc := newExportDecisionService(env, audit)

	const actorID = int64(4242)
	data, err := svc.ExportPhase(ctx, env.sourcePhase.ID, actorID, "admin", "pdf", "")
	require.NoError(t, err)
	require.NotNil(t, data)

	// Data half: the seeded request + child come back assembled.
	require.Len(t, data.Rows, 1, "one seeded request")
	require.Len(t, data.Rows[0].Children, 1, "one seeded child")
	assert.Equal(t, env.sourcePhase.ID, data.Phase.ID)

	// Audit half: exactly one row, written in the same call, carrying
	// the counts that match the disclosed payload.
	require.Len(t, audit.entries, 1, "ExportPhase must record exactly one audit row")
	entry := audit.entries[0]
	assert.Equal(t, auditModels.ResourceTypeEnrollmentPhaseExport, entry.ResourceType)
	assert.Equal(t, actorID, entry.ActorAccountID)
	md := entry.GetMetadata()
	assert.Equal(t, env.sourcePhase.ID, md["phase_id"])
	assert.Equal(t, "pdf", md["format"])
	assert.Equal(t, 1, md["request_count"])
	assert.Equal(t, 1, md["child_count"])
}

func TestDecisionService_ExportPhase_AuditFailureBlocksDisclosure(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	submitOneChild(t, env, "export-blocked@example.com", "Mara", "Blocked")

	// The data loads fine, but the audit write fails. ExportPhase must
	// surface the error and return no payload — the handler refuses to
	// serve a file it could not record (no trail, no disclosure).
	audit := &stubAccessLogRepo{err: errors.New("audit down")}
	svc := newExportDecisionService(env, audit)

	data, err := svc.ExportPhase(ctx, env.sourcePhase.ID, int64(4242), "admin", "pdf", "")
	require.Error(t, err, "a failed audit write must fail the whole export")
	assert.Nil(t, data, "no payload may be returned when the audit was not recorded")
}

// The child-status filter must mirror the admin list's status dropdown:
// only children whose own status matches are exported, requests with no
// matching child drop out, and the audit counts reflect the filtered set.
func TestDecisionService_ExportPhase_ChildStatusFilter(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	// Two submissions: both children start "submitted". Approve one so the
	// phase holds a mixed set.
	_, approvedChildID := submitOneChild(t, env, "filter-approved@example.com", "Ada", "Approved")
	submitOneChild(t, env, "filter-open@example.com", "Ben", "Open")
	require.NoError(t, env.repos.RequestChild.UpdateStatus(
		ctx, approvedChildID, enrollmentModels.ChildStatusApproved, nil, 0))

	auditAll := &stubAccessLogRepo{}
	all, err := newExportDecisionService(env, auditAll).
		ExportPhase(ctx, env.sourcePhase.ID, 4242, "admin", "xlsx", "")
	require.NoError(t, err)
	_, allChildren := all.Counts()
	assert.Equal(t, 2, allChildren, "no filter exports every child")

	// Filter to approved → only Ada, and the audit child_count matches.
	auditApproved := &stubAccessLogRepo{}
	approved, err := newExportDecisionService(env, auditApproved).
		ExportPhase(ctx, env.sourcePhase.ID, 4242, "admin", "xlsx", enrollmentModels.ChildStatusApproved)
	require.NoError(t, err)
	reqCount, childCount := approved.Counts()
	assert.Equal(t, 1, reqCount, "only the request with an approved child remains")
	assert.Equal(t, 1, childCount)
	require.Len(t, approved.Rows, 1)
	require.Len(t, approved.Rows[0].Children, 1)
	assert.Equal(t, "Ada", approved.Rows[0].Children[0].Child.FirstName)
	// Audit row reflects the filtered (disclosed) scope.
	require.Len(t, auditApproved.entries, 1)
	md := auditApproved.entries[0].GetMetadata()
	assert.Equal(t, enrollmentModels.ChildStatusApproved, md["status_filter"])
	assert.Equal(t, 1, md["child_count"])

	// Filter to a status no child holds → empty export, zero counts.
	auditRejected := &stubAccessLogRepo{}
	rejected, err := newExportDecisionService(env, auditRejected).
		ExportPhase(ctx, env.sourcePhase.ID, 4242, "admin", "xlsx", enrollmentModels.ChildStatusRejected)
	require.NoError(t, err)
	rReq, rChild := rejected.Counts()
	assert.Equal(t, 0, rReq, "no request has a rejected child")
	assert.Equal(t, 0, rChild)
}
