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

// resolveSuccessorOfferingSource merges the presence-aware split request
// fields with the old template before anything is written (#2147 review
// round 14): omitted fields inherit, provided fields are authoritative, and
// a Zielgruppe away from 'angebot' drops the rule. Without the merge a split
// silently kept the old template's source and filter no matter what the
// request asked for.
func TestResolveSuccessorOfferingSource(t *testing.T) {
	oldOffering := int64(12)
	newOffering := int64(13)
	sourcedOld := func() *activitiesModel.Group {
		return &activitiesModel.Group{
			TargetGroupType:      activitiesModel.TargetGroupTypeAngebot,
			SourceCareOfferingID: &oldOffering,
			SourceGradeLevels:    []int{1, 2},
		}
	}

	t.Run("omitted fields inherit source and filter", func(t *testing.T) {
		in := &TemplateSplitInput{TargetGroupType: activitiesModel.TargetGroupTypeAngebot}
		require.NoError(t, resolveSuccessorOfferingSource(in, sourcedOld()))
		require.NotNil(t, in.SourceCareOfferingID)
		assert.Equal(t, oldOffering, *in.SourceCareOfferingID)
		assert.Equal(t, []int{1, 2}, in.SourceGradeLevels)
	})

	t.Run("provided values win over the stored ones", func(t *testing.T) {
		in := &TemplateSplitInput{
			TargetGroupType:              activitiesModel.TargetGroupTypeAngebot,
			SourceCareOfferingID:         &newOffering,
			SourceCareOfferingIDProvided: true,
			SourceGradeLevels:            []int{3},
			SourceGradeLevelsProvided:    true,
		}
		require.NoError(t, resolveSuccessorOfferingSource(in, sourcedOld()))
		require.NotNil(t, in.SourceCareOfferingID)
		assert.Equal(t, newOffering, *in.SourceCareOfferingID)
		assert.Equal(t, []int{3}, in.SourceGradeLevels)
	})

	t.Run("filter-only change keeps the inherited source", func(t *testing.T) {
		in := &TemplateSplitInput{
			TargetGroupType:           activitiesModel.TargetGroupTypeAngebot,
			SourceGradeLevels:         []int{4},
			SourceGradeLevelsProvided: true,
		}
		require.NoError(t, resolveSuccessorOfferingSource(in, sourcedOld()))
		require.NotNil(t, in.SourceCareOfferingID)
		assert.Equal(t, oldOffering, *in.SourceCareOfferingID)
		assert.Equal(t, []int{4}, in.SourceGradeLevels)
	})

	t.Run("explicit nil clears the source and drags the omitted filter along", func(t *testing.T) {
		in := &TemplateSplitInput{
			TargetGroupType:              activitiesModel.TargetGroupTypeAngebot,
			SourceCareOfferingIDProvided: true,
		}
		require.NoError(t, resolveSuccessorOfferingSource(in, sourcedOld()))
		assert.Nil(t, in.SourceCareOfferingID)
		assert.Nil(t, in.SourceGradeLevels,
			"a source cleared by explicit null must not leave the stored filter behind (DB CHECK)")
	})

	t.Run("a Zielgruppe away from angebot drops the rule", func(t *testing.T) {
		in := &TemplateSplitInput{
			TargetGroupType:              activitiesModel.TargetGroupTypeNone,
			SourceCareOfferingID:         &newOffering,
			SourceCareOfferingIDProvided: true,
		}
		require.NoError(t, resolveSuccessorOfferingSource(in, sourcedOld()))
		assert.Nil(t, in.SourceCareOfferingID)
		assert.Nil(t, in.SourceGradeLevels)
	})

	t.Run("merged source next to explicit student_ids is rejected", func(t *testing.T) {
		in := &TemplateSplitInput{
			TargetGroupType: activitiesModel.TargetGroupTypeAngebot,
			StudentIDs:      []int64{21},
		}
		err := resolveSuccessorOfferingSource(in, sourcedOld())
		require.ErrorIs(t, err, ErrOfferingSourceInvalid)
		assert.ErrorContains(t, err, "student_ids must be empty")
	})

	t.Run("no stored source stays no source", func(t *testing.T) {
		in := &TemplateSplitInput{TargetGroupType: activitiesModel.TargetGroupTypeAngebot}
		require.NoError(t, resolveSuccessorOfferingSource(in, &activitiesModel.Group{
			TargetGroupType: activitiesModel.TargetGroupTypeAngebot,
		}))
		assert.Nil(t, in.SourceCareOfferingID)
		assert.Nil(t, in.SourceGradeLevels)
	})
}

// offeringRosterFeedChanged gates the split's roster resync: an unchanged
// feed keeps the plain carry-over, every difference triggers reconciliation.
func TestOfferingRosterFeedChanged(t *testing.T) {
	offeringA := int64(12)
	offeringB := int64(13)
	group := func(offeringID *int64, levels []int) *activitiesModel.Group {
		return &activitiesModel.Group{SourceCareOfferingID: offeringID, SourceGradeLevels: levels}
	}

	tests := []struct {
		name        string
		old, newG   *activitiesModel.Group
		wantChanged bool
	}{
		{"both without source", group(nil, nil), group(nil, nil), false},
		{"identical source and filter", group(&offeringA, []int{1, 2}), group(&offeringA, []int{1, 2}), false},
		{"identical filter in different order", group(&offeringA, []int{2, 1}), group(&offeringA, []int{1, 2}), false},
		{"source added", group(nil, nil), group(&offeringA, nil), true},
		{"source removed", group(&offeringA, nil), group(nil, nil), true},
		{"source switched", group(&offeringA, nil), group(&offeringB, nil), true},
		{"filter changed", group(&offeringA, []int{1}), group(&offeringA, []int{2}), true},
		{"filter cleared", group(&offeringA, []int{1}), group(&offeringA, nil), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantChanged, offeringRosterFeedChanged(tc.old, tc.newG))
		})
	}
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
