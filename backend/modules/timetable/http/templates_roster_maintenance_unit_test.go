package timetablehttp

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rosterMaintenanceSources answers the roster indicator from fixed results
// and records what the template list asked for. Enrollment derives the
// indicator itself (services/enrollment DeriveTemplateRosterMaintenance).
type rosterMaintenanceSources struct {
	stubOfferingSources
	results map[int64]timetable.TemplateRosterMaintenance
	err     error
	queries []timetable.TemplateRosterMaintenanceQuery
	calls   int
}

func (s *rosterMaintenanceSources) TemplateRosterMaintenance(
	_ context.Context,
	queries []timetable.TemplateRosterMaintenanceQuery,
) (map[int64]timetable.TemplateRosterMaintenance, error) {
	s.calls++
	s.queries = queries
	return s.results, s.err
}

func TestAttachRosterMaintenance_DerivesEveryTemplateInOneRead(t *testing.T) {
	t.Parallel()

	const (
		classTemplateID   int64 = 101
		sourcedTemplateID int64 = 102
		linkedTemplateID  int64 = 103
	)
	schoolClass := "2a"
	calendarPeriodID := int64(42)
	schedulePeriodID := int64(43)
	sources := &rosterMaintenanceSources{results: map[int64]timetable.TemplateRosterMaintenance{
		classTemplateID: {Mode: "manual", DynamicTargetsManual: true},
		sourcedTemplateID: {
			Mode:        "automatic",
			Offerings:   []timetable.OfferingRef{{ID: 7, Name: "Bis 16 Uhr"}},
			GradeLevels: []int{2},
		},
		linkedTemplateID: {
			Mode:                 "partial",
			Offerings:            []timetable.OfferingRef{{ID: 3, Name: "Mittagessen"}},
			InactiveOfferings:    []timetable.OfferingRef{{ID: 4, Name: "Spätbetreuung"}},
			DynamicTargetsManual: true,
		},
	}}
	resource := NewResource(Dependencies{OfferingSourceOptions: sources})
	templates := []templateResponse{
		{
			ID:              classTemplateID,
			TargetGroupType: timetable.TargetGroupTypeSchoolClass,
			Targets:         []templateTargetResponse{{Type: timetable.TargetGroupTypeSchoolClass, SchoolClass: &schoolClass}},
		},
		{
			ID:                    sourcedTemplateID,
			CalendarPeriodID:      &calendarPeriodID,
			TargetGroupType:       timetable.TargetGroupTypeOffering,
			SourceCareOfferingIDs: []int64{7},
			SourceGradeLevels:     []int{2},
			Schedules: []templateScheduleResponse{{
				CalendarPeriodID: &schedulePeriodID,
			}},
		},
		{
			ID:              linkedTemplateID,
			TargetGroupType: timetable.TargetGroupTypeEducationGroup,
			Targets:         []templateTargetResponse{{Type: timetable.TargetGroupTypeEducationGroup}},
		},
	}

	resource.attachRosterMaintenance(context.Background(), templates, &schedulePeriodID)

	assert.Equal(t, 1, sources.calls, "the indicator must not cost one read per template")
	require.Len(t, sources.queries, 3)
	assert.True(t, sources.queries[0].HasDynamicTargets,
		"a class target resolves once per created occurrence")
	assert.Equal(t, []int64{7}, sources.queries[1].SourceCareOfferingIDs)
	assert.Equal(t, []int{2}, sources.queries[1].SourceGradeLevels)
	assert.False(t, sources.queries[1].HasDynamicTargets)
	assert.Equal(t, &schedulePeriodID, sources.queries[1].CalendarPeriodID,
		"a schedule period pin defines the visible template period")
	assert.True(t, sources.queries[2].HasDynamicTargets,
		"a group target resolves once per created occurrence")

	require.NotNil(t, templates[0].RosterMaintenance)
	assert.Equal(t, "manual", templates[0].RosterMaintenance.Mode)
	assert.True(t, templates[0].RosterMaintenance.DynamicTargets)
	assert.Equal(t, []templateRosterMaintenanceOfferingResponse{}, templates[0].RosterMaintenance.Offerings)

	require.NotNil(t, templates[1].RosterMaintenance)
	assert.Equal(t, "automatic", templates[1].RosterMaintenance.Mode)
	assert.Equal(t, []templateRosterMaintenanceOfferingResponse{{ID: 7, Name: "Bis 16 Uhr"}}, templates[1].RosterMaintenance.Offerings)
	assert.Equal(t, []int{2}, templates[1].RosterMaintenance.GradeLevels)

	require.NotNil(t, templates[2].RosterMaintenance)
	assert.Equal(t, "partial", templates[2].RosterMaintenance.Mode)
	assert.Equal(t, []templateRosterMaintenanceOfferingResponse{{ID: 4, Name: "Spätbetreuung"}}, templates[2].RosterMaintenance.InactiveOfferings)
}

func TestAttachRosterMaintenance_ReadFailureOmitsIndicator(t *testing.T) {
	t.Parallel()

	sources := &rosterMaintenanceSources{err: errors.New("database unavailable")}
	resource := NewResource(Dependencies{OfferingSourceOptions: sources})
	templates := []templateResponse{{ID: 201}}
	periodID := int64(42)

	resource.attachRosterMaintenance(context.Background(), templates, &periodID)

	assert.Nil(t, templates[0].RosterMaintenance,
		"a failed read must not claim a maintenance mode")
}

func TestAttachRosterMaintenance_PeriodFreeReadOmitsIndicator(t *testing.T) {
	t.Parallel()

	periodID := int64(42)
	sources := &rosterMaintenanceSources{results: map[int64]timetable.TemplateRosterMaintenance{}}
	resource := NewResource(Dependencies{OfferingSourceOptions: sources})
	templates := []templateResponse{{
		ID:                    202,
		SourceCareOfferingIDs: []int64{7},
		Schedules:             []templateScheduleResponse{{CalendarPeriodID: &periodID}},
	}}

	resource.attachRosterMaintenance(context.Background(), templates, nil)

	assert.Zero(t, sources.calls, "a period-free read has no unambiguous maintenance state")
	assert.Nil(t, templates[0].RosterMaintenance)
}

func TestAttachRosterMaintenance_UsesVisibleCalendarPeriod(t *testing.T) {
	t.Parallel()

	schedulePeriodID := int64(43)
	templatePeriodID := int64(42)
	requestedPeriodID := int64(44)
	sources := &rosterMaintenanceSources{results: map[int64]timetable.TemplateRosterMaintenance{}}
	resource := NewResource(Dependencies{OfferingSourceOptions: sources})
	templates := []templateResponse{
		{
			ID:                    301,
			CalendarPeriodID:      &templatePeriodID,
			SourceCareOfferingIDs: []int64{7},
			Schedules: []templateScheduleResponse{{
				CalendarPeriodID: &schedulePeriodID,
			}, {
				CalendarPeriodID: &requestedPeriodID,
			}},
		},
		{ID: 302, CalendarPeriodID: &templatePeriodID, SourceCareOfferingIDs: []int64{8}},
		{ID: 303, SourceCareOfferingIDs: []int64{9}},
	}

	resource.attachRosterMaintenance(context.Background(), templates, &requestedPeriodID)

	require.Len(t, sources.queries, 3)
	assert.Equal(t, &requestedPeriodID, sources.queries[0].CalendarPeriodID,
		"the schedule matching the requested period is the visible one")
	assert.Equal(t, &templatePeriodID, sources.queries[1].CalendarPeriodID,
		"the template pin is used when schedules have no pin")
	assert.Equal(t, &requestedPeriodID, sources.queries[2].CalendarPeriodID,
		"a period-scoped read supplies the visible period when nothing is pinned")
}
