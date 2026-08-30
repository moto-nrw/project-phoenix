package enrollment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// gradeCountsByOffering flattens the repository result into a lookup keyed by
// offering id, with the nil-grade bucket under key 0. Tests assert on this
// instead of slice order.
func gradeCountsByOffering(
	rows []*enrollmentModels.CareOfferingGradeLevelCount,
) map[int64]map[int]int {
	out := map[int64]map[int]int{}
	for _, row := range rows {
		if out[row.CareOfferingID] == nil {
			out[row.CareOfferingID] = map[int]int{}
		}
		grade := 0
		if row.GradeLevel != nil {
			grade = int(*row.GradeLevel)
		}
		out[row.CareOfferingID][grade] += row.Count
	}
	return out
}

func int16Ptr(v int16) *int16 { return &v }

func TestRequestChildOfferingRepository_CountActiveGradeLevels_GroupsByGrade(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	from := timezone.NewDate(2026, 8, 24)
	until := from.AddDays(90)

	// Two graded children on one offering, so the grouping has something to
	// separate. The fixture's own child stays out of this offering.
	requestID := requestIDOf(t, db, tenantID, childID)
	grade1Child := addGradedChild(t, db, tenantID, requestID, "Lina", int16Ptr(1), enrollmentModels.ChildStatusSubmitted)
	grade3Child := addGradedChild(t, db, tenantID, requestID, "Mira", int16Ptr(3), enrollmentModels.ChildStatusApproved)

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		if err := repo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: grade1Child,
			CareOfferingID: offeringID,
		}); err != nil {
			return err
		}
		return repo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: grade3Child,
			CareOfferingID: offeringID,
		})
	}))

	var rows []*enrollmentModels.CareOfferingGradeLevelCount
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		rows, err = repo.CountActiveGradeLevelsByCareOfferingIDs(ctx, []int64{offeringID}, from, until)
		return err
	}))

	assert.Equal(t, map[int]int{1: 1, 3: 1}, gradeCountsByOffering(rows)[offeringID])
}

func TestRequestChildOfferingRepository_CountActiveGradeLevels_ReportsMissingGradeSeparately(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	from := timezone.NewDate(2026, 8, 24)
	until := from.AddDays(90)

	// makeChild leaves TargetGradeLevel nil. An availability rule never
	// matches a missing grade, so the caller must be able to see this bucket.
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: childID,
			CareOfferingID: offeringID,
		})
	}))

	var rows []*enrollmentModels.CareOfferingGradeLevelCount
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		rows, err = repo.CountActiveGradeLevelsByCareOfferingIDs(ctx, []int64{offeringID}, from, until)
		return err
	}))

	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].GradeLevel)
	assert.Equal(t, 1, rows[0].Count)
}

func TestRequestChildOfferingRepository_CountActiveGradeLevels_CountsAChildOncePerOffering(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	from := timezone.NewDate(2026, 8, 24)
	until := from.AddDays(90)
	laterFrom := from.AddDays(30)

	// Two non-overlapping intervals for the same child: one booking, not two.
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		if err := repo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: childID,
			CareOfferingID: offeringID,
			ValidUntil:     &laterFrom,
		}); err != nil {
			return err
		}
		return repo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: childID,
			CareOfferingID: offeringID,
			ValidFrom:      &laterFrom,
		})
	}))

	var rows []*enrollmentModels.CareOfferingGradeLevelCount
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		rows, err = repo.CountActiveGradeLevelsByCareOfferingIDs(ctx, []int64{offeringID}, from, until)
		return err
	}))

	require.Len(t, rows, 1)
	assert.Equal(t, 1, rows[0].Count, "one child holding two intervals is one booking")
}

func TestRequestChildOfferingRepository_CountActiveGradeLevels_ExcludesTerminalChildren(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	from := timezone.NewDate(2026, 8, 24)
	until := from.AddDays(90)

	requestID := requestIDOf(t, db, tenantID, childID)
	withdrawn := addGradedChild(t, db, tenantID, requestID, "Nele", int16Ptr(2), enrollmentModels.ChildStatusWithdrawn)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: withdrawn,
			CareOfferingID: offeringID,
		})
	}))

	var rows []*enrollmentModels.CareOfferingGradeLevelCount
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		rows, err = repo.CountActiveGradeLevelsByCareOfferingIDs(ctx, []int64{offeringID}, from, until)
		return err
	}))

	assert.Empty(t, rows, "a withdrawn child holds no slot")
}

func TestRequestChildOfferingRepository_CountActiveGradeLevels_ExcludesIntervalsOutsideTheWindow(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	from := timezone.NewDate(2026, 8, 24)
	endedAt := from.AddDays(-10)
	startedAt := from.AddDays(-40)

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: childID,
			CareOfferingID: offeringID,
			ValidFrom:      &startedAt,
			ValidUntil:     &endedAt,
		})
	}))

	var rows []*enrollmentModels.CareOfferingGradeLevelCount
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		rows, err = repo.CountActiveGradeLevelsByCareOfferingIDs(ctx, []int64{offeringID}, from, from.AddDays(90))
		return err
	}))

	assert.Empty(t, rows, "an interval that ended before the window is history")
}

func TestRequestChildOfferingRepository_CountActiveGradeLevels_RejectsAnEmptyWindow(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, _, offeringID := setupChildOfferingTest(t)
	today := timezone.NewDate(2026, 8, 24)

	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		_, listErr := repo.CountActiveGradeLevelsByCareOfferingIDs(ctx, []int64{offeringID}, today, today)
		return listErr
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestRequestChildOfferingRepository_CountActiveGradeLevels_EmptyInputSkipsTheQuery(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, _, _ := setupChildOfferingTest(t)
	today := timezone.NewDate(2026, 8, 24)

	var rows []*enrollmentModels.CareOfferingGradeLevelCount
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		// An empty window would error if the query ran; it must not.
		rows, err = repo.CountActiveGradeLevelsByCareOfferingIDs(ctx, nil, today, today)
		return err
	}))

	assert.Empty(t, rows)
}

// The batched peak query must agree with the single-offering variant the
// capacity gate uses — a display that disagrees with the gate is worse than
// no display. They count the same population unconditionally, including
// across phases (see Aggregates_CountEveryPhaseLikeTheGate).
func TestRequestChildOfferingRepository_CountMaxActiveByIDsInRange_MatchesTheSingleOfferingVariant(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	from := timezone.NewDate(2026, 8, 24)
	until := from.AddDays(90)
	requestID := requestIDOf(t, db, tenantID, childID)
	second := addGradedChild(t, db, tenantID, requestID, "Mira", int16Ptr(2), enrollmentModels.ChildStatusApproved)

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		if err := repo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: childID, CareOfferingID: offeringID,
		}); err != nil {
			return err
		}
		return repo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: second, CareOfferingID: offeringID,
		})
	}))

	var single int
	var batched map[int64]int
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		single, err = repo.CountMaxActiveByCareOfferingInRange(ctx, offeringID, from, until)
		if err != nil {
			return err
		}
		batched, err = repo.CountMaxActiveByCareOfferingIDsInRange(ctx, []int64{offeringID}, from, until)
		return err
	}))

	assert.Equal(t, 2, single)
	assert.Equal(t, single, batched[offeringID])
}

func TestRequestChildOfferingRepository_CountMaxActiveByIDsInRange_SeparatesOfferings(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	phaseID := phaseIDOfOffering(t, db, tenantID, offeringID)
	from := timezone.NewDate(2026, 8, 24)
	until := from.AddDays(90)

	// A second offering in the same phase, deliberately left unbooked.
	emptyOfferingID := addSiblingOffering(t, db, tenantID, phaseID, "batchpeak")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: childID, CareOfferingID: offeringID,
		})
	}))

	var batched map[int64]int
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		batched, err = repo.CountMaxActiveByCareOfferingIDsInRange(ctx, []int64{offeringID, emptyOfferingID}, from, until)
		return err
	}))

	assert.Equal(t, 1, batched[offeringID])
	_, present := batched[emptyOfferingID]
	assert.False(t, present, "an offering with no bookings is absent, callers read that as zero")
}

func TestRequestChildOfferingRepository_CountMaxActiveByIDsInRange_GuardsItsInput(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, _, offeringID := setupChildOfferingTest(t)
	today := timezone.NewDate(2026, 8, 24)

	var empty map[int64]int
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		empty, err = repo.CountMaxActiveByCareOfferingIDsInRange(ctx, nil, today, today)
		return err
	}))
	assert.Empty(t, empty, "empty input short-circuits before the empty-range guard")

	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		_, rangeErr := repo.CountMaxActiveByCareOfferingIDsInRange(ctx, []int64{offeringID}, today, today)
		return rangeErr
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

// #2186 review round 2: a care_offering row can be reached from two phases at
// once (legacy rows from rollovers predating the offering remap in #2249), and
// capacity lives on that row — both bookings occupy the same slots. The
// capacity gate has always counted them together, so the batched aggregates
// that feed the admin dialog must too. Scoping them to one phase made the
// dialog show a free slot the gate then refused as full.
func TestRequestChildOfferingRepository_Aggregates_CountEveryPhaseLikeTheGate(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	from := timezone.NewDate(2026, 8, 24)
	until := from.AddDays(90)

	// A child of ANOTHER phase holding the same offering row.
	_, _ = addRolloverSuccessorHolding(t, db, tenantID, offeringID, int16Ptr(4))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: childID, CareOfferingID: offeringID,
		})
	}))

	var gate int
	var peak map[int64]int
	var grades []*enrollmentModels.CareOfferingGradeLevelCount
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		gate, err = repo.CountMaxActiveByCareOfferingInRange(ctx, offeringID, from, until)
		if err != nil {
			return err
		}
		peak, err = repo.CountMaxActiveByCareOfferingIDsInRange(ctx, []int64{offeringID}, from, until)
		if err != nil {
			return err
		}
		grades, err = repo.CountActiveGradeLevelsByCareOfferingIDs(ctx, []int64{offeringID}, from, until)
		return err
	}))

	assert.Equal(t, 2, gate, "both phases' children occupy the shared offering")
	assert.Equal(t, gate, peak[offeringID],
		"the displayed occupancy must equal what the capacity gate enforces")

	total := 0
	rolled := false
	for _, row := range grades {
		total += row.Count
		if row.GradeLevel != nil && *row.GradeLevel == 4 {
			rolled = true
		}
	}
	assert.Equal(t, 2, total, "the grade-level hint covers the same population")
	assert.True(t, rolled, "the other phase's booking is subject to the same availability rule")
}
