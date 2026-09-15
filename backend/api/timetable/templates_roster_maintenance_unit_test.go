package timetable

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rosterMaintenanceLister struct {
	stubOfferingSourceLister
	feeds   map[int64]enrollmentSvc.TemplateRosterFeeds
	err     error
	queries []enrollmentSvc.TemplateRosterFeedQuery
	calls   int
}

func (s *rosterMaintenanceLister) TemplateRosterMaintenanceFeeds(
	_ context.Context,
	queries []enrollmentSvc.TemplateRosterFeedQuery,
) (map[int64]enrollmentSvc.TemplateRosterFeeds, error) {
	s.calls++
	s.queries = queries
	return s.feeds, s.err
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
	lister := &rosterMaintenanceLister{feeds: map[int64]enrollmentSvc.TemplateRosterFeeds{
		classTemplateID: {CareOfferingsEnabled: true},
		sourcedTemplateID: {
			CareOfferingsEnabled: true,
			Sources:              []enrollmentSvc.TemplateRosterFeedOffering{{ID: 7, Name: "Bis 16 Uhr", IsActive: true}},
		},
		linkedTemplateID: {
			CareOfferingsEnabled: true,
			LinkedOfferings:      []enrollmentSvc.TemplateRosterFeedOffering{{ID: 3, Name: "Mittagessen", IsActive: true}},
		},
	}}
	resource := NewResource(Dependencies{OfferingSourceOptions: lister})
	templates := []templateResponse{
		{
			ID:              classTemplateID,
			TargetGroupType: activities.TargetGroupTypeKlasse,
			Targets:         []templateTargetResponse{{Type: activities.TargetGroupTypeKlasse, SchoolClass: &schoolClass}},
		},
		{
			ID:                    sourcedTemplateID,
			CalendarPeriodID:      &calendarPeriodID,
			TargetGroupType:       activities.TargetGroupTypeAngebot,
			SourceCareOfferingIDs: []int64{7},
			SourceGradeLevels:     []int{2},
			Schedules: []templateScheduleResponse{{
				CalendarPeriodID: &schedulePeriodID,
			}},
		},
		{
			ID:              linkedTemplateID,
			TargetGroupType: activities.TargetGroupTypeGruppe,
			Targets:         []templateTargetResponse{{Type: activities.TargetGroupTypeGruppe}},
		},
	}

	resource.attachRosterMaintenance(context.Background(), templates, nil)

	assert.Equal(t, 1, lister.calls, "the indicator must not cost one read per template")
	require.Len(t, lister.queries, 3)
	assert.Equal(t, []int64{7}, lister.queries[1].SourceCareOfferingIDs)
	assert.Equal(t, &schedulePeriodID, lister.queries[1].CalendarPeriodID,
		"a schedule period pin defines the visible template period")

	require.NotNil(t, templates[0].RosterMaintenance)
	assert.Equal(t, "manual", templates[0].RosterMaintenance.Mode,
		"a class target without an offering is not maintained automatically")
	assert.True(t, templates[0].RosterMaintenance.DynamicTargets)

	require.NotNil(t, templates[1].RosterMaintenance)
	assert.Equal(t, "automatic", templates[1].RosterMaintenance.Mode)
	assert.Equal(t, []templateRosterMaintenanceOfferingResponse{{ID: 7, Name: "Bis 16 Uhr"}}, templates[1].RosterMaintenance.Offerings)
	assert.Equal(t, []int{2}, templates[1].RosterMaintenance.GradeLevels)

	require.NotNil(t, templates[2].RosterMaintenance)
	assert.Equal(t, "partial", templates[2].RosterMaintenance.Mode)
}

func TestAttachRosterMaintenance_ReadFailureOmitsIndicator(t *testing.T) {
	t.Parallel()

	lister := &rosterMaintenanceLister{err: errors.New("database unavailable")}
	resource := NewResource(Dependencies{OfferingSourceOptions: lister})
	templates := []templateResponse{{ID: 201}}

	resource.attachRosterMaintenance(context.Background(), templates, nil)

	assert.Nil(t, templates[0].RosterMaintenance,
		"a failed read must not claim a maintenance mode")
}

func TestAttachRosterMaintenance_UsesVisibleCalendarPeriod(t *testing.T) {
	t.Parallel()

	schedulePeriodID := int64(43)
	templatePeriodID := int64(42)
	requestedPeriodID := int64(44)
	lister := &rosterMaintenanceLister{feeds: map[int64]enrollmentSvc.TemplateRosterFeeds{}}
	resource := NewResource(Dependencies{OfferingSourceOptions: lister})
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

	require.Len(t, lister.queries, 3)
	assert.Equal(t, &requestedPeriodID, lister.queries[0].CalendarPeriodID,
		"the schedule matching the requested period is the visible one")
	assert.Equal(t, &templatePeriodID, lister.queries[1].CalendarPeriodID,
		"the template pin is used when schedules have no pin")
	assert.Equal(t, &requestedPeriodID, lister.queries[2].CalendarPeriodID,
		"a period-scoped read supplies the visible period when nothing is pinned")
}
