package schedule

import (
	"context"
	"errors"
	"fmt"
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
	t.Parallel()

	tests := []struct {
		name               string
		sourceOfferingIDs  []int64
		gradeLevels        []int
		schoolClasses      []string
		targetGroupType    string
		studentIDs         []int64
		weekdayAssignments []WeekdayRosterAssignment
		wantErr            string
	}{
		{
			name:              "source with grade filter on an Angebot is valid",
			sourceOfferingIDs: []int64{12},
			gradeLevels:       []int{1, 2},
			targetGroupType:   activitiesModel.TargetGroupTypeAngebot,
		},
		{
			name:              "source without grade filter is valid",
			sourceOfferingIDs: []int64{12},
			targetGroupType:   activitiesModel.TargetGroupTypeAngebot,
		},
		{
			name:            "no source and no filter is valid",
			targetGroupType: activitiesModel.TargetGroupTypeNone,
		},
		{
			name:            "grade filter without a source is rejected",
			gradeLevels:     []int{1},
			targetGroupType: activitiesModel.TargetGroupTypeAngebot,
			wantErr:         "source_grade_levels requires source_care_offering_ids",
		},
		{
			name:              "non-positive offering id is rejected",
			sourceOfferingIDs: []int64{0},
			targetGroupType:   activitiesModel.TargetGroupTypeAngebot,
			wantErr:           "source_care_offering_ids entries must be positive",
		},
		{
			name:              "duplicate offering ids are rejected",
			sourceOfferingIDs: []int64{12, 12},
			targetGroupType:   activitiesModel.TargetGroupTypeAngebot,
			wantErr:           "source_care_offering_ids must not contain duplicates",
		},
		{
			name:              "several distinct offerings are valid",
			sourceOfferingIDs: []int64{12, 13, 14},
			gradeLevels:       []int{1},
			targetGroupType:   activitiesModel.TargetGroupTypeAngebot,
		},
		{
			name:              "source outside the Angebot Zielgruppe is rejected",
			sourceOfferingIDs: []int64{12},
			targetGroupType:   activitiesModel.TargetGroupTypeNone,
			wantErr:           "requires target group type 'angebot'",
		},
		{
			name:              "out-of-range grade filter is rejected",
			sourceOfferingIDs: []int64{12},
			gradeLevels:       []int{0, 2},
			targetGroupType:   activitiesModel.TargetGroupTypeAngebot,
			wantErr:           "source_grade_levels entries must be between",
		},
		{
			name:              "grade filter above the supported bound is rejected",
			sourceOfferingIDs: []int64{12},
			gradeLevels:       []int{14},
			targetGroupType:   activitiesModel.TargetGroupTypeAngebot,
			wantErr:           "source_grade_levels entries must be between",
		},
		{
			name:              "duplicate grade filter entries are rejected",
			sourceOfferingIDs: []int64{12},
			gradeLevels:       []int{2, 2},
			targetGroupType:   activitiesModel.TargetGroupTypeAngebot,
			wantErr:           "source_grade_levels must not contain duplicates",
		},
		{
			name:              "source with class filter on an Angebot is valid",
			sourceOfferingIDs: []int64{12},
			schoolClasses:     []string{"1a", "1b"},
			targetGroupType:   activitiesModel.TargetGroupTypeAngebot,
		},
		{
			name:            "class filter without a source is rejected",
			schoolClasses:   []string{"1a"},
			targetGroupType: activitiesModel.TargetGroupTypeAngebot,
			wantErr:         "source_school_classes requires source_care_offering_ids",
		},
		{
			name:              "class and grade filter together are rejected",
			sourceOfferingIDs: []int64{12},
			gradeLevels:       []int{1},
			schoolClasses:     []string{"1a"},
			targetGroupType:   activitiesModel.TargetGroupTypeAngebot,
			wantErr:           "source_school_classes and source_grade_levels cannot be combined",
		},
		{
			name:              "duplicate class filter entries are rejected",
			sourceOfferingIDs: []int64{12},
			schoolClasses:     []string{"1a", " 1A "},
			targetGroupType:   activitiesModel.TargetGroupTypeAngebot,
			wantErr:           "source_school_classes must not contain duplicates",
		},
		{
			name:              "manual child list next to a source is rejected",
			sourceOfferingIDs: []int64{12},
			targetGroupType:   activitiesModel.TargetGroupTypeAngebot,
			studentIDs:        []int64{21},
			wantErr:           "student_ids must be empty",
		},
		{
			name:              "per-weekday roster next to a source is rejected",
			sourceOfferingIDs: []int64{12},
			targetGroupType:   activitiesModel.TargetGroupTypeAngebot,
			weekdayAssignments: []WeekdayRosterAssignment{{
				Weekday: activitiesModel.WeekdayMonday,
			}},
			wantErr: "weekday_assignments must be empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOfferingSourceInput(
				tc.sourceOfferingIDs,
				tc.gradeLevels,
				tc.schoolClasses,
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
	t.Parallel()

	t.Run("create", func(t *testing.T) {
		err := validateTemplateCreateInput(CreateTemplateInput{
			Name:                  "Frühbetreuung",
			Weekdays:              []int{activitiesModel.WeekdayMonday},
			CategoryID:            22,
			RoomID:                33,
			RosterValidFrom:       timezone.NewDate(2026, 8, 10),
			TargetGroupType:       activitiesModel.TargetGroupTypeNone,
			SourceCareOfferingIDs: []int64{12},
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
		assert.ErrorContains(t, err, "source_grade_levels requires source_care_offering_ids")
	})
}

// resolveSuccessorOfferingSource merges the presence-aware split request
// fields with the old template before anything is written (#2147 review
// round 14): omitted fields inherit, provided fields are authoritative, and
// a Zielgruppe away from 'angebot' drops the rule. Without the merge a split
// silently kept the old template's source and filter no matter what the
// request asked for.
func TestResolveSuccessorOfferingSource(t *testing.T) {
	t.Parallel()

	sourcedOld := func() *activitiesModel.Group {
		return &activitiesModel.Group{
			TargetGroupType:       activitiesModel.TargetGroupTypeAngebot,
			SourceCareOfferingIDs: []int64{12, 15},
			SourceGradeLevels:     []int{1, 2},
		}
	}

	t.Run("omitted fields inherit sources and filter", func(t *testing.T) {
		in := &TemplateSplitInput{TargetGroupType: activitiesModel.TargetGroupTypeAngebot}
		require.NoError(t, resolveSuccessorOfferingSource(in, sourcedOld()))
		assert.Equal(t, []int64{12, 15}, in.SourceCareOfferingIDs)
		assert.Equal(t, []int{1, 2}, in.SourceGradeLevels)
	})

	t.Run("provided values win over the stored ones", func(t *testing.T) {
		in := &TemplateSplitInput{
			TargetGroupType:               activitiesModel.TargetGroupTypeAngebot,
			SourceCareOfferingIDs:         []int64{13},
			SourceCareOfferingIDsProvided: true,
			SourceGradeLevels:             []int{3},
			SourceGradeLevelsProvided:     true,
		}
		require.NoError(t, resolveSuccessorOfferingSource(in, sourcedOld()))
		assert.Equal(t, []int64{13}, in.SourceCareOfferingIDs)
		assert.Equal(t, []int{3}, in.SourceGradeLevels)
	})

	t.Run("filter-only change keeps the inherited sources", func(t *testing.T) {
		in := &TemplateSplitInput{
			TargetGroupType:           activitiesModel.TargetGroupTypeAngebot,
			SourceGradeLevels:         []int{4},
			SourceGradeLevelsProvided: true,
		}
		require.NoError(t, resolveSuccessorOfferingSource(in, sourcedOld()))
		assert.Equal(t, []int64{12, 15}, in.SourceCareOfferingIDs)
		assert.Equal(t, []int{4}, in.SourceGradeLevels)
	})

	t.Run("explicit nil clears the sources and drags the omitted filter along", func(t *testing.T) {
		in := &TemplateSplitInput{
			TargetGroupType:               activitiesModel.TargetGroupTypeAngebot,
			SourceCareOfferingIDsProvided: true,
		}
		require.NoError(t, resolveSuccessorOfferingSource(in, sourcedOld()))
		assert.Nil(t, in.SourceCareOfferingIDs)
		assert.Nil(t, in.SourceGradeLevels,
			"a source cleared by explicit null must not leave the stored filter behind (DB CHECK)")
	})

	t.Run("a Zielgruppe away from angebot drops the rule", func(t *testing.T) {
		in := &TemplateSplitInput{
			TargetGroupType:               activitiesModel.TargetGroupTypeNone,
			SourceCareOfferingIDs:         []int64{13},
			SourceCareOfferingIDsProvided: true,
		}
		require.NoError(t, resolveSuccessorOfferingSource(in, sourcedOld()))
		assert.Nil(t, in.SourceCareOfferingIDs)
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
		assert.Nil(t, in.SourceCareOfferingIDs)
		assert.Nil(t, in.SourceGradeLevels)
	})
}

// offeringRosterFeedChanged gates the split's roster resync: an unchanged
// feed keeps the plain carry-over, every difference triggers reconciliation.
func TestOfferingRosterFeedChanged(t *testing.T) {
	t.Parallel()

	group := func(offeringIDs []int64, levels []int) *activitiesModel.Group {
		return &activitiesModel.Group{SourceCareOfferingIDs: offeringIDs, SourceGradeLevels: levels}
	}

	tests := []struct {
		name        string
		old, newG   *activitiesModel.Group
		wantChanged bool
	}{
		{"both without source", group(nil, nil), group(nil, nil), false},
		{"identical source and filter", group([]int64{12}, []int{1, 2}), group([]int64{12}, []int{1, 2}), false},
		{"identical filter in different order", group([]int64{12}, []int{2, 1}), group([]int64{12}, []int{1, 2}), false},
		{"identical multi-source set", group([]int64{12, 13}, nil), group([]int64{12, 13}, nil), false},
		{"source added", group(nil, nil), group([]int64{12}, nil), true},
		{"source removed", group([]int64{12}, nil), group(nil, nil), true},
		{"source switched", group([]int64{12}, nil), group([]int64{13}, nil), true},
		{"second source added", group([]int64{12}, nil), group([]int64{12, 13}, nil), true},
		{"source order changed", group([]int64{12, 13}, nil), group([]int64{13, 12}, nil), true},
		{"filter changed", group([]int64{12}, []int{1}), group([]int64{12}, []int{2}), true},
		{"filter cleared", group([]int64{12}, []int{1}), group([]int64{12}, nil), true},
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
	t.Parallel()

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

		require.NoError(t, svc.resyncUpdatedTemplateOfferingRoster(t.Context(), baseInput(), []int64{11}, nil))

		assert.Equal(t, int64(77), got.TemplateID)
		assert.Empty(t, got.OfferingIDs, "a cleared source must reach the hook as empty")
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
		in.Fields.SourceCareOfferingIDs = []int64{12, 13}
		in.Fields.SourceGradeLevels = []int{1, 2}

		require.NoError(t, svc.resyncUpdatedTemplateOfferingRoster(t.Context(), in, nil, &scheduleFrom))

		assert.Equal(t, []int64{12, 13}, got.OfferingIDs)
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
		in.Fields.SourceCareOfferingIDs = []int64{12}
		pastStart := timezone.TodayDate().AddDays(-30)

		require.NoError(t, svc.resyncUpdatedTemplateOfferingRoster(t.Context(), in, nil, &pastStart))

		assert.Equal(t, timezone.TodayDate(), got.EffectiveFrom,
			"#2147 review: a past schedule start must not become the rewrite boundary — that would delete or cap roster rows that were already effective")
	})

	t.Run("a missing hook fails loudly instead of saving a dead rule", func(t *testing.T) {
		svc := NewTimetableDataService(TimetableDataDependencies{})
		in := baseInput()
		in.Fields.SourceCareOfferingIDs = []int64{12}

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
		in.Fields.SourceCareOfferingIDs = []int64{12}

		err := svc.resyncUpdatedTemplateOfferingRoster(t.Context(), in, nil, nil)

		require.ErrorIs(t, err, sentinel)
		assert.ErrorContains(t, err, "update template: resync offering roster")
	})
}

// validateOfferingSourceReference is the pre-write guard that keeps an unknown
// or period-incompatible source from reaching the group insert/update, where
// the FK would turn a client mistake into a 500 (#2147 review round 18).
func TestValidateOfferingSourceReference(t *testing.T) {
	t.Parallel()

	periodID := int64(55)

	t.Run("no source skips the hook", func(t *testing.T) {
		called := false
		svc := NewTimetableDataService(TimetableDataDependencies{
			ValidateOfferingSource: func(context.Context, []int64, []int64, *int64) error {
				called = true
				return nil
			},
		})

		require.NoError(t, svc.validateOfferingSourceReference(t.Context(), nil, nil, &periodID, "create template: validate offering source"))
		assert.False(t, called, "a template without a source must not resolve an offering")
	})

	t.Run("the sources are checked against the template period", func(t *testing.T) {
		var gotOfferings []int64
		var gotPeriod *int64
		svc := NewTimetableDataService(TimetableDataDependencies{
			ValidateOfferingSource: func(_ context.Context, ids, _ []int64, period *int64) error {
				gotOfferings, gotPeriod = ids, period
				return nil
			},
		})

		require.NoError(t, svc.validateOfferingSourceReference(t.Context(), []int64{12, 13}, nil, &periodID, "create template: validate offering source"))
		assert.Equal(t, []int64{12, 13}, gotOfferings)
		assert.Equal(t, &periodID, gotPeriod)
	})

	t.Run("an unknown source surfaces as ErrOfferingSourceInvalid before any write", func(t *testing.T) {
		svc := NewTimetableDataService(TimetableDataDependencies{
			ValidateOfferingSource: func(context.Context, []int64, []int64, *int64) error {
				return fmt.Errorf("%w: care offering %d not found", ErrOfferingSourceInvalid, int64(12))
			},
		})

		err := svc.validateOfferingSourceReference(t.Context(), []int64{12}, nil, nil, "update template: validate offering source")

		require.ErrorIs(t, err, ErrOfferingSourceInvalid,
			"the handler maps this sentinel to 400 — an FK violation during the write would render 500 instead")
		assert.ErrorContains(t, err, "update template: validate offering source")
	})

	t.Run("an unwired hook leaves the resync as the only guard", func(t *testing.T) {
		svc := NewTimetableDataService(TimetableDataDependencies{})

		require.NoError(t, svc.validateOfferingSourceReference(t.Context(), []int64{12}, nil, &periodID, "create template: validate offering source"))
	})
}
