package enrollmenthttp_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestListPublicPhasesHandler_DoesNotLeakOtherTenantPhases(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	now := time.Now().UnixNano()
	ctx := context.Background()
	org := &platformModels.Organization{
		ID:     now,
		Name:   "Public Phase Tenant Scope Org",
		Slug:   fmt.Sprintf("public-phase-tenant-scope-org-%d", now),
		Active: true,
	}
	testpkg.CreateTestOrganization(t, db, org)

	targetSchool := publicPhaseSchool{
		ID:   now + 100,
		Slug: fmt.Sprintf("target-public-phase-school-%d", now),
	}
	otherSchool := publicPhaseSchool{
		ID:   now + 200,
		Slug: fmt.Sprintf("other-public-phase-school-%d", now),
	}
	for _, school := range []struct {
		publicPhaseSchool
		name string
	}{{targetSchool, "Target Public Phase School"}, {otherSchool, "Other Public Phase School"}} {
		_, err := db.NewRaw(`INSERT INTO platform.schools (id, organization_id, name, slug, subdomain, active) VALUES (?, ?, ?, ?, ?, true)`,
			school.ID, org.ID, school.name, school.Slug, school.Slug).Exec(ctx)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		_, _ = db.NewRaw(`DELETE FROM enrollment.phases WHERE tenant_id IN (?, ?)`, targetSchool.ID, otherSchool.ID).Exec(context.Background())
		_, _ = db.NewRaw(`DELETE FROM platform.schools WHERE id IN (?, ?)`, targetSchool.ID, otherSchool.ID).Exec(context.Background())
		_, _ = db.NewRaw(`DELETE FROM platform.organizations WHERE id = ?`, org.ID).Exec(context.Background())
	})

	phaseRepo := NewTestModule()
	targetPhase := &capability.Phase{ID: now + 300, Name: "Target Tenant Phase", Kind: capability.PhaseKindSchoolYear, ServiceStartDate: "2026-09-01", ServiceEndDate: "2027-07-31", IsActive: true, CareOverflowMode: capability.PhaseCareOverflowWaitlist}
	otherPhase := &capability.Phase{ID: now + 400, Name: "Other Tenant Phase", Kind: capability.PhaseKindSchoolYear, ServiceStartDate: "2026-09-01", ServiceEndDate: "2027-07-31", IsActive: true, CareOverflowMode: capability.PhaseCareOverflowWaitlist}
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, targetSchool.ID, func(txCtx context.Context, _ bun.Tx) error {
		return phaseRepo.InsertPhase(txCtx, targetPhase)
	}))
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, otherSchool.ID, func(txCtx context.Context, _ bun.Tx) error {
		return phaseRepo.InsertPhase(txCtx, otherPhase)
	}))

	resource := enrollmentAPI.NewResource(
		nil, nil, nil, nil,
		NewTestPhases(TestPhaseDependencies{Records: NewTestModule()}),
		nil, nil, nil, nil, nil, enrollmentAPI.GuardianInvitationRuntime{}, nil, dbPublicSchools{db: db},
	)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/phases/public/%s", targetSchool.Slug), nil)
	req = req.WithContext(testpkg.WithPackageTenantRuntime(req.Context()))
	w := httptest.NewRecorder()
	resource.Router().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"name":"Target Tenant Phase"`)
	assert.NotContains(t, w.Body.String(), `"name":"Other Tenant Phase"`)
}

type publicPhaseSchool struct {
	ID   int64
	Slug string
}

// dbPublicSchools resolves a public enrollment slug straight from the seeded
// platform.schools rows.
type dbPublicSchools struct{ db *bun.DB }

func (d dbPublicSchools) GetSchoolBySlug(ctx context.Context, slug string) (*enrollmentAPI.PublicSchool, error) {
	var ids []int64
	err := d.db.NewSelect().
		TableExpr(`platform.schools AS "school"`).
		Column("school.id").
		Where(`"school".slug = ?`, slug).
		Where(`"school".deleted_at IS NULL`).
		Scan(ctx, &ids)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	return &enrollmentAPI.PublicSchool{ID: ids[0]}, nil
}
