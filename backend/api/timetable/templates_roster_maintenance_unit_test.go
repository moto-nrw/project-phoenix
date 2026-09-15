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
			TargetGroupType:       activities.TargetGroupTypeAngebot,
			SourceCareOfferingIDs: []int64{7},
			SourceGradeLevels:     []int{2},
		},
		{
			ID:              linkedTemplateID,
			TargetGroupType: activities.TargetGroupTypeGruppe,
			Targets:         []templateTargetResponse{{Type: activities.TargetGroupTypeGruppe}},
		},
	}

	resource.attachRosterMaintenance(context.Background(), templates)

	assert.Equal(t, 1, lister.calls, "the indicator must not cost one read per template")
	require.Len(t, lister.queries, 3)
	assert.Equal(t, []int64{7}, lister.queries[1].SourceCareOfferingIDs)

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

	resource.attachRosterMaintenance(context.Background(), templates)

	assert.Nil(t, templates[0].RosterMaintenance,
		"a failed read must not claim a maintenance mode")
}
