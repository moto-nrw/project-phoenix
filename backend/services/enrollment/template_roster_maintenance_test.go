package enrollment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestDeriveTemplateRosterMaintenance(t *testing.T) {
	t.Parallel()

	active := func(id int64, name string) enrollmentService.TemplateRosterFeedOffering {
		return enrollmentService.TemplateRosterFeedOffering{ID: id, Name: name, IsActive: true}
	}
	inactive := func(id int64, name string) enrollmentService.TemplateRosterFeedOffering {
		return enrollmentService.TemplateRosterFeedOffering{ID: id, Name: name}
	}

	t.Run("class target without offering is manual", func(t *testing.T) {
		t.Parallel()
		// The support case of #3140: a Randstunde for a class, no offering.
		got := enrollmentService.DeriveTemplateRosterMaintenance(enrollmentService.TemplateRosterMaintenanceInput{
			HasDynamicTargets: true,
			Feeds:             enrollmentService.TemplateRosterFeeds{CareOfferingsEnabled: true},
		})
		assert.Equal(t, enrollmentService.RosterMaintenanceManual, got.Mode)
		assert.True(t, got.DynamicTargetsManual)
		assert.Empty(t, got.Offerings)
		assert.False(t, got.CareOfferingsDisabled)
	})

	t.Run("plain manual roster is manual", func(t *testing.T) {
		t.Parallel()
		got := enrollmentService.DeriveTemplateRosterMaintenance(enrollmentService.TemplateRosterMaintenanceInput{
			Feeds: enrollmentService.TemplateRosterFeeds{CareOfferingsEnabled: true},
		})
		assert.Equal(t, enrollmentService.RosterMaintenanceManual, got.Mode)
		assert.False(t, got.DynamicTargetsManual)
	})

	t.Run("source offering with class filter is automatic", func(t *testing.T) {
		t.Parallel()
		got := enrollmentService.DeriveTemplateRosterMaintenance(enrollmentService.TemplateRosterMaintenanceInput{
			SourceCareOfferingIDs: []int64{7},
			SourceSchoolClasses:   []string{"2a"},
			Feeds: enrollmentService.TemplateRosterFeeds{
				CareOfferingsEnabled: true,
				Sources:              []enrollmentService.TemplateRosterFeedOffering{active(7, "Bis 16 Uhr")},
			},
		})
		assert.Equal(t, enrollmentService.RosterMaintenanceAutomatic, got.Mode)
		assert.Equal(t, []enrollmentService.RosterMaintenanceOffering{{ID: 7, Name: "Bis 16 Uhr"}}, got.Offerings)
		assert.Equal(t, []string{"2a"}, got.SchoolClasses)
	})

	t.Run("offering link on the offering is automatic", func(t *testing.T) {
		t.Parallel()
		got := enrollmentService.DeriveTemplateRosterMaintenance(enrollmentService.TemplateRosterMaintenanceInput{
			Feeds: enrollmentService.TemplateRosterFeeds{
				CareOfferingsEnabled: true,
				LinkedOfferings:      []enrollmentService.TemplateRosterFeedOffering{active(3, "Mittagessen")},
			},
		})
		assert.Equal(t, enrollmentService.RosterMaintenanceAutomatic, got.Mode)
		assert.Equal(t, []enrollmentService.RosterMaintenanceOffering{{ID: 3, Name: "Mittagessen"}}, got.Offerings)
		assert.Empty(t, got.GradeLevels, "a linked offering carries no source filter")
	})

	t.Run("invalid offering link is manual", func(t *testing.T) {
		t.Parallel()
		got := enrollmentService.DeriveTemplateRosterMaintenance(enrollmentService.TemplateRosterMaintenanceInput{
			Feeds: enrollmentService.TemplateRosterFeeds{
				CareOfferingsEnabled: true,
				LinkedOfferings: []enrollmentService.TemplateRosterFeedOffering{{
					ID: 3, Name: "Mittagessen", IsActive: true, IsInvalid: true,
				}},
			},
		})
		assert.Equal(t, enrollmentService.RosterMaintenanceManual, got.Mode)
		assert.Equal(t, []enrollmentService.RosterMaintenanceOffering{{ID: 3, Name: "Mittagessen"}}, got.InvalidOfferings)
	})

	t.Run("offering link plus class target is partial", func(t *testing.T) {
		t.Parallel()
		got := enrollmentService.DeriveTemplateRosterMaintenance(enrollmentService.TemplateRosterMaintenanceInput{
			HasDynamicTargets: true,
			Feeds: enrollmentService.TemplateRosterFeeds{
				CareOfferingsEnabled: true,
				LinkedOfferings:      []enrollmentService.TemplateRosterFeedOffering{active(3, "Mittagessen")},
			},
		})
		assert.Equal(t, enrollmentService.RosterMaintenancePartial, got.Mode)
		assert.True(t, got.DynamicTargetsManual)
	})

	t.Run("inactive offering link is named", func(t *testing.T) {
		t.Parallel()
		got := enrollmentService.DeriveTemplateRosterMaintenance(enrollmentService.TemplateRosterMaintenanceInput{
			Feeds: enrollmentService.TemplateRosterFeeds{
				CareOfferingsEnabled: true,
				LinkedOfferings:      []enrollmentService.TemplateRosterFeedOffering{inactive(3, "Mittagessen")},
			},
		})
		assert.Equal(t, enrollmentService.RosterMaintenanceManual, got.Mode)
		assert.Equal(t, []enrollmentService.RosterMaintenanceOffering{{ID: 3, Name: "Mittagessen"}}, got.InactiveOfferings)
	})

	t.Run("inactive offering link beside an active link is partial", func(t *testing.T) {
		t.Parallel()
		got := enrollmentService.DeriveTemplateRosterMaintenance(enrollmentService.TemplateRosterMaintenanceInput{
			Feeds: enrollmentService.TemplateRosterFeeds{
				CareOfferingsEnabled: true,
				LinkedOfferings: []enrollmentService.TemplateRosterFeedOffering{
					active(3, "Mittagessen"), inactive(4, "Spätbetreuung"),
				},
			},
		})
		assert.Equal(t, enrollmentService.RosterMaintenancePartial, got.Mode)
		assert.Equal(t, []enrollmentService.RosterMaintenanceOffering{{ID: 4, Name: "Spätbetreuung"}}, got.InactiveOfferings)
	})

	t.Run("inactive offering is not automatic", func(t *testing.T) {
		t.Parallel()
		got := enrollmentService.DeriveTemplateRosterMaintenance(enrollmentService.TemplateRosterMaintenanceInput{
			SourceCareOfferingIDs: []int64{7, 8},
			SourceGradeLevels:     []int{2},
			Feeds: enrollmentService.TemplateRosterFeeds{
				CareOfferingsEnabled: true,
				Sources: []enrollmentService.TemplateRosterFeedOffering{
					active(7, "Bis 16 Uhr"), inactive(8, "Musik"),
				},
				LinkedOfferings: []enrollmentService.TemplateRosterFeedOffering{inactive(9, "Alt")},
			},
		})
		assert.Equal(t, enrollmentService.RosterMaintenanceManual, got.Mode,
			"one inactive source stops the whole source rule")
		assert.Empty(t, got.Offerings)
		assert.Empty(t, got.GradeLevels)
		assert.Equal(t, []enrollmentService.RosterMaintenanceOffering{
			{ID: 8, Name: "Musik"},
			{ID: 9, Name: "Alt"},
		}, got.InactiveOfferings)
	})

	t.Run("broken source beside a working link is partial", func(t *testing.T) {
		t.Parallel()
		got := enrollmentService.DeriveTemplateRosterMaintenance(enrollmentService.TemplateRosterMaintenanceInput{
			SourceCareOfferingIDs: []int64{8},
			Feeds: enrollmentService.TemplateRosterFeeds{
				CareOfferingsEnabled: true,
				Sources:              []enrollmentService.TemplateRosterFeedOffering{inactive(8, "Musik")},
				LinkedOfferings:      []enrollmentService.TemplateRosterFeedOffering{active(3, "Mittagessen")},
			},
		})
		assert.Equal(t, enrollmentService.RosterMaintenancePartial, got.Mode)
		assert.Equal(t, []enrollmentService.RosterMaintenanceOffering{{ID: 3, Name: "Mittagessen"}}, got.Offerings)
	})

	t.Run("same offering as source and link is named once", func(t *testing.T) {
		t.Parallel()
		got := enrollmentService.DeriveTemplateRosterMaintenance(enrollmentService.TemplateRosterMaintenanceInput{
			SourceCareOfferingIDs: []int64{7},
			Feeds: enrollmentService.TemplateRosterFeeds{
				CareOfferingsEnabled: true,
				Sources:              []enrollmentService.TemplateRosterFeedOffering{active(7, "Bis 16 Uhr")},
				LinkedOfferings:      []enrollmentService.TemplateRosterFeedOffering{active(7, "Bis 16 Uhr")},
			},
		})
		assert.Equal(t, enrollmentService.RosterMaintenanceAutomatic, got.Mode)
		assert.Len(t, got.Offerings, 1)
	})

	t.Run("switched-off care offerings are manual", func(t *testing.T) {
		t.Parallel()
		got := enrollmentService.DeriveTemplateRosterMaintenance(enrollmentService.TemplateRosterMaintenanceInput{
			SourceCareOfferingIDs: []int64{7},
			Feeds: enrollmentService.TemplateRosterFeeds{
				Sources: []enrollmentService.TemplateRosterFeedOffering{active(7, "Bis 16 Uhr")},
			},
		})
		assert.Equal(t, enrollmentService.RosterMaintenanceManual, got.Mode)
		assert.True(t, got.CareOfferingsDisabled)
		assert.Empty(t, got.Offerings)
	})

	t.Run("vanished source ids leave a manual roster", func(t *testing.T) {
		t.Parallel()
		// The reader drops ids whose offering no longer exists, like every resync.
		got := enrollmentService.DeriveTemplateRosterMaintenance(enrollmentService.TemplateRosterMaintenanceInput{
			SourceCareOfferingIDs: []int64{404},
			Feeds:                 enrollmentService.TemplateRosterFeeds{CareOfferingsEnabled: true},
		})
		assert.Equal(t, enrollmentService.RosterMaintenanceManual, got.Mode)
		assert.False(t, got.CareOfferingsDisabled)
	})
}

func TestTemplateRosterMaintenanceFeeds_ResolvesSourcesLinksAndSeries(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	sourceOffering := createSourceOffering(t, env, "FeedQuelle", nil)
	inactiveOffering := createSourceOffering(t, env, "FeedInaktiv", nil)
	_, err := env.db.NewRaw(`UPDATE enrollment.care_offerings SET is_active = FALSE WHERE id = ?`, inactiveOffering.ID).Exec(ctx)
	require.NoError(t, err)

	sourced := createSourcedTemplate(t, env, "FeedSourced", sourceOffering.ID, []int{2}, period)
	linkedRoot := createCareOfferingTemplateGroup(t, env.db, "FeedLinkedRoot")
	successor := createCareOfferingTemplateGroup(t, env.db, "FeedLinkedSuccessor")
	_, err = env.db.NewRaw(`UPDATE activities.groups SET series_root_id = ? WHERE id = ?`, linkedRoot.ID, successor.ID).Exec(ctx)
	require.NoError(t, err)
	linkedOffering := createSourceOffering(t, env, "FeedVerknuepft", &linkedRoot.ID)
	manual := createCareOfferingTemplateGroup(t, env.db, "FeedManual")

	reader, ok := env.decision.(enrollmentService.OfferingSourceOptionLister)
	require.True(t, ok)
	queries := []enrollmentService.TemplateRosterFeedQuery{
		{TemplateID: sourced.ID, SourceCareOfferingIDs: []int64{sourceOffering.ID, inactiveOffering.ID, 987654321}},
		{TemplateID: successor.ID},
		{TemplateID: manual.ID},
	}
	feeds, err := reader.TemplateRosterMaintenanceFeeds(ctx, queries)
	require.NoError(t, err)

	sourcedFeeds := feeds[sourced.ID]
	assert.True(t, sourcedFeeds.CareOfferingsEnabled, "registry default is on")
	require.Len(t, sourcedFeeds.Sources, 2, "the vanished id is dropped")
	assert.Equal(t, sourceOffering.ID, sourcedFeeds.Sources[0].ID)
	assert.True(t, sourcedFeeds.Sources[0].IsActive)
	assert.Equal(t, sourceOffering.Name, sourcedFeeds.Sources[0].Name)
	assert.Equal(t, inactiveOffering.ID, sourcedFeeds.Sources[1].ID)
	assert.False(t, sourcedFeeds.Sources[1].IsActive)
	assert.Empty(t, sourcedFeeds.LinkedOfferings)

	successorFeeds := feeds[successor.ID]
	require.Len(t, successorFeeds.LinkedOfferings, 1,
		"a link on the original segment feeds every split successor")
	assert.Equal(t, linkedOffering.ID, successorFeeds.LinkedOfferings[0].ID)

	manualFeeds := feeds[manual.ID]
	assert.Empty(t, manualFeeds.Sources)
	assert.Empty(t, manualFeeds.LinkedOfferings)
}

func TestTemplateRosterMaintenanceFeeds_ReportsSwitchedOffCareOfferings(t *testing.T) {
	t.Parallel()

	disabled := false
	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{careOfferingsEnabled: &disabled})
	defer cleanup()

	template := createCareOfferingTemplateGroup(t, env.db, "FeedDisabled")
	reader, ok := env.decision.(enrollmentService.OfferingSourceOptionLister)
	require.True(t, ok)
	feeds, err := reader.TemplateRosterMaintenanceFeeds(testpkg.Ctx(t), []enrollmentService.TemplateRosterFeedQuery{{TemplateID: template.ID}})
	require.NoError(t, err)
	assert.False(t, feeds[template.ID].CareOfferingsEnabled)
}

func TestTemplateRosterMaintenanceFeeds_RejectsSourceOutsideTemplatePeriod(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	source := createSourceOffering(t, env, "FeedAusserhalbZeitraum", nil)
	period := createCareOfferingTestPeriod(t, env.db, "feed-too-short",
		timezone.Date(env.sourcePhase.ServiceStartDate),
		timezone.Date(env.sourcePhase.ServiceEndDate).AddDays(-1))
	reader, ok := env.decision.(enrollmentService.OfferingSourceOptionLister)
	require.True(t, ok)

	feeds, err := reader.TemplateRosterMaintenanceFeeds(ctx, []enrollmentService.TemplateRosterFeedQuery{{
		TemplateID:            987654321,
		CalendarPeriodID:      &period.ID,
		SourceCareOfferingIDs: []int64{source.ID},
	}})
	require.NoError(t, err)

	maintenance := enrollmentService.DeriveTemplateRosterMaintenance(enrollmentService.TemplateRosterMaintenanceInput{
		Feeds: enrollmentService.TemplateRosterFeeds{
			CareOfferingsEnabled: feeds[987654321].CareOfferingsEnabled,
			Sources:              feeds[987654321].Sources,
		},
	})
	assert.Equal(t, enrollmentService.RosterMaintenanceManual, maintenance.Mode)
	assert.Equal(t, []enrollmentService.RosterMaintenanceOffering{{ID: source.ID, Name: source.Name}}, maintenance.InvalidOfferings)
}

func TestTemplateRosterMaintenanceFeeds_RejectsLinkedOfferingOutsideTemplatePeriod(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := createCareOfferingTestPeriod(t, env.db, "linked-feed-too-short",
		timezone.Date(env.sourcePhase.ServiceStartDate),
		timezone.Date(env.sourcePhase.ServiceEndDate).AddDays(-1))
	template := createCareOfferingTemplateGroup(t, env.db, "FeedLinkedAusserhalbZeitraum")
	template.CalendarPeriodID = &period.ID
	require.NoError(t, env.repos.ActivityGroup.Update(ctx, template))
	createCareOfferingTemplateSchedule(t, env.db, template.ID, activitiesModels.WeekdayMonday, &period.ID)
	linked := createSourceOffering(t, env, "FeedLinkedAusserhalbZeitraum", &template.ID)
	reader, ok := env.decision.(enrollmentService.OfferingSourceOptionLister)
	require.True(t, ok)

	feeds, err := reader.TemplateRosterMaintenanceFeeds(ctx, []enrollmentService.TemplateRosterFeedQuery{{
		TemplateID:       template.ID,
		CalendarPeriodID: &period.ID,
	}})
	require.NoError(t, err)

	maintenance := enrollmentService.DeriveTemplateRosterMaintenance(enrollmentService.TemplateRosterMaintenanceInput{
		Feeds: feeds[template.ID],
	})
	assert.Equal(t, enrollmentService.RosterMaintenanceManual, maintenance.Mode)
	assert.Equal(t, []enrollmentService.RosterMaintenanceOffering{{ID: linked.ID, Name: linked.Name}}, maintenance.InvalidOfferings)
}

func TestTemplateRosterMaintenanceFeeds_RejectsSourcesFromDifferentPhases(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	first := createSourceOffering(t, env, "FeedErstePhase", nil)
	second := createSourceOffering(t, env, "FeedZweitePhase", nil)
	otherPhase := *env.sourcePhase
	otherPhase.ID = 0
	otherPhase.Name = uniqueSchemaName("FeedAnderePhase-" + t.Name())
	require.NoError(t, enrollmentTest.New().InsertPhase(ctx, &otherPhase))
	_, err := env.db.NewRaw(`UPDATE enrollment.care_offerings SET phase_id = ? WHERE id = ?`, otherPhase.ID, second.ID).Exec(ctx)
	require.NoError(t, err)

	reader, ok := env.decision.(enrollmentService.OfferingSourceOptionLister)
	require.True(t, ok)
	feeds, err := reader.TemplateRosterMaintenanceFeeds(ctx, []enrollmentService.TemplateRosterFeedQuery{{
		TemplateID:            987654322,
		CalendarPeriodID:      &period.ID,
		SourceCareOfferingIDs: []int64{first.ID, second.ID},
	}})
	require.NoError(t, err)

	maintenance := enrollmentService.DeriveTemplateRosterMaintenance(enrollmentService.TemplateRosterMaintenanceInput{
		Feeds: feeds[987654322],
	})
	assert.Equal(t, enrollmentService.RosterMaintenanceManual, maintenance.Mode,
		"resync rejects source offerings from different enrollment phases")
	assert.Equal(t, []enrollmentService.RosterMaintenanceOffering{
		{ID: first.ID, Name: first.Name},
		{ID: second.ID, Name: second.Name},
	}, maintenance.InvalidOfferings,
		"the explanation names every source rejected by the mixed-phase rule")
}
