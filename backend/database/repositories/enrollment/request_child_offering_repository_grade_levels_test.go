package enrollment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
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
	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	from := timezone.TodayDate()
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
	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	from := timezone.TodayDate()
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
	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	from := timezone.TodayDate()
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
	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	from := timezone.TodayDate()
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
	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	from := timezone.TodayDate()
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
	db, repo, tenantID, _, offeringID := setupChildOfferingTest(t)
	today := timezone.TodayDate()

	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		_, listErr := repo.CountActiveGradeLevelsByCareOfferingIDs(ctx, []int64{offeringID}, today, today)
		return listErr
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestRequestChildOfferingRepository_CountActiveGradeLevels_EmptyInputSkipsTheQuery(t *testing.T) {
	db, repo, tenantID, _, _ := setupChildOfferingTest(t)
	today := timezone.TodayDate()

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
// no display (#2186 review).
func TestRequestChildOfferingRepository_CountMaxActiveByIDsInRange_MatchesTheSingleOfferingVariant(t *testing.T) {
	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	from := timezone.TodayDate()
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
	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	from := timezone.TodayDate()
	until := from.AddDays(90)

	// A second offering in the same phase, deliberately left unbooked.
	offeringRepo := enrollmentRepo.NewCareOfferingRepository(db)
	var emptyOfferingID int64
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		booked, err := offeringRepo.FindByID(ctx, offeringID)
		if err != nil {
			return err
		}
		other := makeOffering(booked.PhaseID, uniqueOfferingName("batchpeak"))
		if err := offeringRepo.Create(ctx, other); err != nil {
			return err
		}
		emptyOfferingID = other.ID
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
	db, repo, tenantID, _, offeringID := setupChildOfferingTest(t)
	today := timezone.TodayDate()

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
