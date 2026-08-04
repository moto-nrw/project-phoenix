package schedule

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
)

// Issue #2137: the offering-source request contract is shared by template
// create and update. It is pure input validation, so it is pinned here
// without a database.
func TestValidateOfferingSourceInput(t *testing.T) {
	offeringID := int64(12)
	zeroOffering := int64(0)

	tests := []struct {
		name               string
		sourceOfferingID   *int64
		gradeLevels        []int
		targetGroupType    string
		studentIDs         []int64
		weekdayAssignments []WeekdayRosterAssignment
		wantErr            string
	}{
		{
			name:             "source with grade filter on an Angebot is valid",
			sourceOfferingID: &offeringID,
			gradeLevels:      []int{1, 2},
			targetGroupType:  activitiesModel.TargetGroupTypeAngebot,
		},
		{
			name:             "source without grade filter is valid",
			sourceOfferingID: &offeringID,
			targetGroupType:  activitiesModel.TargetGroupTypeAngebot,
		},
		{
			name:            "no source and no filter is valid",
			targetGroupType: activitiesModel.TargetGroupTypeNone,
		},
		{
			name:            "grade filter without a source is rejected",
			gradeLevels:     []int{1},
			targetGroupType: activitiesModel.TargetGroupTypeAngebot,
			wantErr:         "source_grade_levels requires source_care_offering_id",
		},
		{
			name:             "non-positive offering id is rejected",
			sourceOfferingID: &zeroOffering,
			targetGroupType:  activitiesModel.TargetGroupTypeAngebot,
			wantErr:          "source_care_offering_id must be positive",
		},
		{
			name:             "source outside the Angebot Zielgruppe is rejected",
			sourceOfferingID: &offeringID,
			targetGroupType:  activitiesModel.TargetGroupTypeNone,
			wantErr:          "requires target group type 'angebot'",
		},
		{
			name:             "out-of-range grade filter is rejected",
			sourceOfferingID: &offeringID,
			gradeLevels:      []int{0, 2},
			targetGroupType:  activitiesModel.TargetGroupTypeAngebot,
			wantErr:          "source_grade_levels entries must be between",
		},
		{
			name:             "grade filter above the supported bound is rejected",
			sourceOfferingID: &offeringID,
			gradeLevels:      []int{14},
			targetGroupType:  activitiesModel.TargetGroupTypeAngebot,
			wantErr:          "source_grade_levels entries must be between",
		},
		{
			name:             "duplicate grade filter entries are rejected",
			sourceOfferingID: &offeringID,
			gradeLevels:      []int{2, 2},
			targetGroupType:  activitiesModel.TargetGroupTypeAngebot,
			wantErr:          "source_grade_levels must not contain duplicates",
		},
		{
			name:             "manual child list next to a source is rejected",
			sourceOfferingID: &offeringID,
			targetGroupType:  activitiesModel.TargetGroupTypeAngebot,
			studentIDs:       []int64{21},
			wantErr:          "student_ids must be empty",
		},
		{
			name:             "per-weekday roster next to a source is rejected",
			sourceOfferingID: &offeringID,
			targetGroupType:  activitiesModel.TargetGroupTypeAngebot,
			weekdayAssignments: []WeekdayRosterAssignment{{
				Weekday: activitiesModel.WeekdayMonday,
			}},
			wantErr: "weekday_assignments must be empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOfferingSourceInput(
				tc.sourceOfferingID,
				tc.gradeLevels,
				tc.targetGroupType,
				tc.studentIDs,
				tc.weekdayAssignments,
			)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, ErrOfferingSourceInvalid)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// The create and update entry points must both run the shared contract —
// a rule accepted on one path and rejected on the other would let the editor
// save a template it cannot re-save.
func TestTemplateInputValidationRunsTheOfferingSourceContract(t *testing.T) {
	offeringID := int64(12)

	t.Run("create", func(t *testing.T) {
		err := validateTemplateCreateInput(CreateTemplateInput{
			Name:                 "Frühbetreuung",
			Weekdays:             []int{activitiesModel.WeekdayMonday},
			CategoryID:           22,
			RoomID:               33,
			RosterValidFrom:      timezone.NewDate(2026, 8, 10),
			TargetGroupType:      activitiesModel.TargetGroupTypeNone,
			SourceCareOfferingID: &offeringID,
		})

		assert.ErrorIs(t, err, ErrOfferingSourceInvalid)
		assert.ErrorContains(t, err, "requires target group type 'angebot'")
	})

	t.Run("update", func(t *testing.T) {
		err := validateTemplateUpdateInput(TemplateUpdateInput{
			TemplateID:      77,
			TimeframeID:     44,
			Weekdays:        []int{activitiesModel.WeekdayMonday},
			RosterValidFrom: timezone.NewDate(2026, 8, 10),
			GradeLevelMax:   4,
			Fields: activitiesModel.TemplateFieldsUpdate{
				TargetGroupType:   activitiesModel.TargetGroupTypeAngebot,
				SourceGradeLevels: []int{1},
			},
		})

		assert.ErrorIs(t, err, ErrOfferingSourceInvalid)
		assert.ErrorContains(t, err, "source_grade_levels requires source_care_offering_id")
	})
}

// resyncUpdatedTemplateOfferingRoster decides whether an edit has to touch
// the offering-sourced roster at all, and with which window.
//
// The fixture dates are today-relative on purpose: since the #2147 review the
// boundary is clamped to today, so fixed calendar dates would flip these
// assertions once real time passes them.
func TestResyncUpdatedTemplateOfferingRoster(t *testing.T) {
	offeringID := int64(12)
	previousID := int64(11)
	periodID := int64(55)
	rosterFrom := timezone.TodayDate().AddDays(6)
	scheduleFrom := timezone.TodayDate().AddDays(28)

	baseInput := func() TemplateUpdateInput {
		return TemplateUpdateInput{
			TemplateID:       77,
			CalendarPeriodID: &periodID,
			RosterValidFrom:  rosterFrom,
		}
	}

	t.Run("a template without a source before and after skips the hook", func(t *testing.T) {
		called := false
		svc := NewTimetableDataService(TimetableDataDependencies{
			ResyncOfferingRoster: func(context.Context, OfferingRosterResyncInput) error {
				called = true
				return nil
			},
		})

		require.NoError(t, svc.resyncUpdatedTemplateOfferingRoster(t.Context(), baseInput(), nil, nil))
		assert.False(t, called, "an edit that never involves a source must not reconcile a roster")
	})

	t.Run("removing a source still reconciles, using the previous offering", func(t *testing.T) {
		var got OfferingRosterResyncInput
		svc := NewTimetableDataService(TimetableDataDependencies{
			ResyncOfferingRoster: func(_ context.Context, in OfferingRosterResyncInput) error {
				got = in
				return nil
			},
		})

		require.NoError(t, svc.resyncUpdatedTemplateOfferingRoster(t.Context(), baseInput(), &previousID, nil))

		assert.Equal(t, int64(77), got.TemplateID)
		assert.Equal(t, &previousID, got.PreviousOfferingID)
		assert.Nil(t, got.OfferingID, "a cleared source must reach the hook as nil")
		assert.Equal(t, &periodID, got.CalendarPeriodID)
		assert.Equal(t, rosterFrom, got.EffectiveFrom)
	})

	t.Run("a series start date wins over the roster valid_from", func(t *testing.T) {
		var got OfferingRosterResyncInput
		svc := NewTimetableDataService(TimetableDataDependencies{
			ResyncOfferingRoster: func(_ context.Context, in OfferingRosterResyncInput) error {
				got = in
				return nil
			},
		})

		in := baseInput()
		in.Fields.SourceCareOfferingID = &offeringID
		in.Fields.SourceGradeLevels = []int{1, 2}

		require.NoError(t, svc.resyncUpdatedTemplateOfferingRoster(t.Context(), in, nil, &scheduleFrom))

		assert.Equal(t, &offeringID, got.OfferingID)
		assert.Equal(t, []int{1, 2}, got.GradeLevels)
		assert.Equal(t, scheduleFrom, got.EffectiveFrom,
			"#2135: the series start bounds the roster rewrite")
	})

	t.Run("an already-started series clamps the rewrite boundary to today", func(t *testing.T) {
		var got OfferingRosterResyncInput
		svc := NewTimetableDataService(TimetableDataDependencies{
			ResyncOfferingRoster: func(_ context.Context, in OfferingRosterResyncInput) error {
				got = in
				return nil
			},
		})

		in := baseInput()
		in.Fields.SourceCareOfferingID = &offeringID
		pastStart := timezone.TodayDate().AddDays(-30)

		require.NoError(t, svc.resyncUpdatedTemplateOfferingRoster(t.Context(), in, nil, &pastStart))

		assert.Equal(t, timezone.TodayDate(), got.EffectiveFrom,
			"#2147 review: a past schedule start must not become the rewrite boundary — that would delete or cap roster rows that were already effective")
	})

	t.Run("a missing hook fails loudly instead of saving a dead rule", func(t *testing.T) {
		svc := NewTimetableDataService(TimetableDataDependencies{})
		in := baseInput()
		in.Fields.SourceCareOfferingID = &offeringID

		err := svc.resyncUpdatedTemplateOfferingRoster(t.Context(), in, nil, nil)

		require.Error(t, err)
		assert.ErrorContains(t, err, "offering roster resync is not configured")
	})

	t.Run("a failing resync surfaces as a schedule error", func(t *testing.T) {
		sentinel := errors.New("boom")
		svc := NewTimetableDataService(TimetableDataDependencies{
			ResyncOfferingRoster: func(context.Context, OfferingRosterResyncInput) error {
				return sentinel
			},
		})
		in := baseInput()
		in.Fields.SourceCareOfferingID = &offeringID

		err := svc.resyncUpdatedTemplateOfferingRoster(t.Context(), in, nil, nil)

		require.ErrorIs(t, err, sentinel)
		assert.ErrorContains(t, err, "update template: resync offering roster")
	})
}
