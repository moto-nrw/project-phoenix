package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/postgres"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type catalogTraeger struct {
	organization int64
	school       int64
}

func newCatalogTraeger(t *testing.T, db *bun.DB, name string) catalogTraeger {
	t.Helper()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: name, Slug: fmt.Sprintf("traeger-katalog-%d", testpkg.UniqueTestTenantID(t)), Active: true,
	}).ID
	testpkg.OwnTraegerCatalog(t, db, organization)
	return catalogTraeger{organization: organization, school: testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")}
}

func traegerPhaseFields(name string) enrollment.TraegerPhaseFields {
	return enrollment.TraegerPhaseFields{
		Name: name, Kind: "school_year", ServiceStartDate: "2027-08-01", ServiceEndDate: "2028-07-31",
		Audience: enrollment.PhaseAudienceOpen, CareOverflowMode: "waitlist", CareOfferingSelectionMode: "optional",
	}
}

// TestTraegerPhaseStatementsKeepTheirTraegerBeneathRLS pins the Träger filter
// of every statement of the Träger phase store (ADR 0195 section 2) beneath
// RLS, as #5952 and #6001 do: in the Träger transaction of A, as the owner of
// the tables, each statement is asked for B's phase. Each assertion goes red
// when its statement drops its organization_id filter.
func TestTraegerPhaseStatementsKeepTheirTraegerBeneathRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	bg := context.Background()
	own := newCatalogTraeger(t, db, "Trägerphase A")
	foreign := newCatalogTraeger(t, db, "Trägerphase B")
	var foreignPhase int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.traeger_phases (organization_id, draft) VALUES (?, '{"name":"Fremd"}') RETURNING id`,
		foreign.organization).Scan(bg, &foreignPhase))
	_, err := db.ExecContext(bg, `INSERT INTO enrollment.traeger_phase_schools (organization_id, traeger_phase_id, school_id) VALUES (?, ?, ?)`,
		foreign.organization, foreignPhase, foreign.school)
	require.NoError(t, err)
	store := postgres.NewTraegerPhaseStore()
	version := 1

	ctx := testpkg.ContextWithTenantRuntime(bg, testpkg.TenantRuntime(t, db))
	require.NoError(t, testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		phases, err := store.ListTraegerPhases(ctx, own.organization)
		require.NoError(t, err)
		assert.Empty(t, phases, "A lists no phase of B")
		_, err = store.TraegerPhase(ctx, own.organization, foreignPhase)
		assert.ErrorIs(t, err, enrollment.ErrTraegerPhaseNotFound, "A reads no phase of B")
		_, err = store.LockTraegerPhase(ctx, own.organization, foreignPhase)
		assert.ErrorIs(t, err, enrollment.ErrTraegerPhaseNotFound, "A locks no phase of B")
		updated, err := store.UpdateTraegerPhaseDraft(ctx, own.organization, foreignPhase, traegerPhaseFields("Übernommen"), true)
		require.NoError(t, err)
		assert.False(t, updated, "A changes no phase of B")
		withdrawn, err := store.WithdrawTraegerPhase(ctx, own.organization, foreignPhase)
		require.NoError(t, err)
		assert.False(t, withdrawn, "A withdraws no phase of B")
		require.NoError(t, store.ReplaceTraegerPhaseSchools(ctx, own.organization, foreignPhase, nil), "A's empty selection reaches no school of B")
		return nil
	}))
	require.NoError(t, testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		err := store.InsertTraegerPhaseVersion(ctx, own.organization, foreignPhase, version, traegerPhaseFields("Übernommen"), 0)
		assert.Error(t, err, "A releases no version of B's phase")
		return nil
	}))

	var draft string
	var selected int
	var released *int
	require.NoError(t, db.NewRaw(`SELECT draft->>'name', released_version FROM enrollment.traeger_phases WHERE id = ?`, foreignPhase).Scan(bg, &draft, &released))
	assert.Equal(t, "Fremd", draft)
	assert.Nil(t, released)
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM enrollment.traeger_phase_schools WHERE traeger_phase_id = ? AND selected`, foreignPhase).Scan(bg, &selected))
	assert.Equal(t, 1, selected, "B keeps its selection")
}

// TestTraegerPhaseStoreRefusesASchoolTransaction pins ADR 0195 section 2
// point 6: the store runs only in the Träger transaction of the Träger it
// names.
func TestTraegerPhaseStoreRefusesASchoolTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newCatalogTraeger(t, db, "Trägerphase Wächter")
	other := newCatalogTraeger(t, db, "Trägerphase Anderer")
	store := postgres.NewTraegerPhaseStore()
	_, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, func(ctx context.Context, organization int64) ([]enrollment.TraegerPhase, error) {
		return store.ListTraegerPhases(ctx, organization)
	}, own.organization)
	assert.ErrorContains(t, err, "Träger transaction of this Träger is required", "a school transaction is refused")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	require.NoError(t, testpkg.WithinTraeger(ctx, other.organization, func(ctx context.Context) error {
		_, err := store.ListTraegerPhases(ctx, own.organization)
		assert.ErrorContains(t, err, "Träger transaction of this Träger is required", "another Träger's transaction is refused")
		return nil
	}))
}

// TestTraegerPhaseLifecycle drives the Träger phase from draft to release
// and outcome: a release writes a version only when the draft changed, a
// school outside the Träger is refused, and a withdrawn phase takes no new
// draft.
func TestTraegerPhaseLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newCatalogTraeger(t, db, "Trägerphase Ablauf")
	foreign := newCatalogTraeger(t, db, "Trägerphase Ablauf Fremd")
	phases := enrollmentCompose.NewTraegerPhases()
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	require.NoError(t, testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		_, err := phases.CreateTraegerPhase(ctx, own.organization, enrollment.TraegerPhaseInput{
			Fields: traegerPhaseFields("Schuljahr"), SchoolIDs: []int64{foreign.school},
		})
		assert.ErrorIs(t, err, enrollment.ErrTraegerSchoolOutsideTraeger, "a school of another Träger is refused")
		return nil
	}))
	require.NoError(t, testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		_, err := phases.CreateTraegerPhase(ctx, own.organization, enrollment.TraegerPhaseInput{
			Fields: enrollment.TraegerPhaseFields{Name: " "}, SchoolIDs: []int64{own.school},
		})
		assert.ErrorIs(t, err, enrollment.ErrInvalidPhase, "a draft no school could store is refused")

		phase, err := phases.CreateTraegerPhase(ctx, own.organization, enrollment.TraegerPhaseInput{
			Fields: traegerPhaseFields("Schuljahr"), SchoolIDs: []int64{own.school},
		})
		require.NoError(t, err)
		require.Len(t, phase.Schools, 1)
		version, err := phases.ReleaseTraegerPhase(ctx, own.organization, phase.ID, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, version)
		version, err = phases.ReleaseTraegerPhase(ctx, own.organization, phase.ID, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, version, "an unchanged draft writes no version")
		_, err = phases.UpdateTraegerPhase(ctx, own.organization, phase.ID, enrollment.TraegerPhaseInput{
			Fields: traegerPhaseFields("Schuljahr 27/28"), SchoolIDs: []int64{own.school},
		})
		require.NoError(t, err)
		version, err = phases.ReleaseTraegerPhase(ctx, own.organization, phase.ID, 0)
		require.NoError(t, err)
		assert.Equal(t, 2, version, "a changed draft writes the next version")

		applied := 2
		require.NoError(t, phases.RecordTraegerPhaseOutcomes(ctx, own.organization, phase.ID, []enrollment.TraegerCopyOutcome{
			{SchoolID: own.school, AppliedVersion: &applied, Outcome: enrollment.TraegerOutcomeApplied},
		}))
		require.NoError(t, phases.RecordTraegerPhaseOutcomes(ctx, own.organization, phase.ID, []enrollment.TraegerCopyOutcome{
			{SchoolID: own.school, Outcome: enrollment.TraegerOutcomeRejected, ErrorCode: "enrollment.phase_invalid"},
		}))
		read, err := phases.TraegerPhase(ctx, own.organization, phase.ID)
		require.NoError(t, err)
		require.Len(t, read.Schools, 1)
		assert.Equal(t, enrollment.TraegerOutcomeRejected, read.Schools[0].Outcome)
		require.NotNil(t, read.Schools[0].AppliedVersion, "a rejection keeps the version applied before")
		assert.Equal(t, 2, *read.Schools[0].AppliedVersion)
		assert.Equal(t, "Schuljahr 27/28", read.Released.Name)

		_, err = phases.UpdateTraegerPhase(ctx, own.organization, phase.ID, enrollment.TraegerPhaseInput{Fields: traegerPhaseFields("Schuljahr 27/28")})
		require.NoError(t, err)
		read, err = phases.TraegerPhase(ctx, own.organization, phase.ID)
		require.NoError(t, err)
		require.Len(t, read.Schools, 1, "a school with a copy stays until the copy is withdrawn")
		assert.False(t, read.Schools[0].Selected)

		_, err = phases.WithdrawTraegerPhase(ctx, own.organization, phase.ID)
		require.NoError(t, err)
		_, err = phases.UpdateTraegerPhase(ctx, own.organization, phase.ID, enrollment.TraegerPhaseInput{Fields: traegerPhaseFields("Neu")})
		assert.ErrorIs(t, err, enrollment.ErrTraegerPhaseWithdrawn)
		return nil
	}))
}
