package compose_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/traegercalendar"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// fixedTraegerSchools names the schools of each Träger of a test.
type fixedTraegerSchools map[int64][]traegercalendar.SchoolState

func (s fixedTraegerSchools) TraegerSchools(_ context.Context, organizationID int64) ([]traegercalendar.SchoolState, error) {
	return s[organizationID], nil
}

type closingDayTraeger struct {
	id      int64
	schools []int64
}

func newClosingDayTraeger(t *testing.T, db *bun.DB, name string, schools int) closingDayTraeger {
	t.Helper()
	slug := fmt.Sprintf("traeger-schliesstag-%d", testpkg.UniqueTestTenantID(t))
	traeger := closingDayTraeger{id: testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID}
	testpkg.OwnTraegerClosingDays(t, db, traeger.id)
	for i := range schools {
		traeger.schools = append(traeger.schools, testpkg.CreateTestSchoolInOrganization(t, db, traeger.id, fmt.Sprintf("%s OGS %d", name, i+1)))
	}
	return traeger
}

func (c closingDayTraeger) states() []traegercalendar.SchoolState {
	states := make([]traegercalendar.SchoolState, 0, len(c.schools))
	for i, id := range c.schools {
		states = append(states, traegercalendar.SchoolState{ID: id, Name: fmt.Sprintf("OGS %d", i+1), FederalState: "DE-BW"})
	}
	return states
}

func planClosingDay(t *testing.T, ctx context.Context, days traegercalendar.ClosingDays, traeger closingDayTraeger, day calendar.Date, reason string, schools ...int64) traegercalendar.ClosingDay {
	t.Helper()
	var planned traegercalendar.ClosingDay
	require.NoError(t, testpkg.WithinTraeger(ctx, traeger.id, func(ctx context.Context) error {
		var err error
		planned, err = days.CreateClosingDay(ctx, traeger.id, traegercalendar.ClosingDayInput{StartDate: day, EndDate: day, Reason: reason, SchoolIDs: schools})
		return err
	}))
	return planned
}

// The Träger's own filter keeps another Träger's closing days out even
// without row level security (rule 25): the Träger transaction drops its
// request role, and reading, changing and deleting still reach only the
// rows of the Träger it names. Removing organization_id from a statement of
// the store turns this test red.
func TestTraegerClosingDaysStayWithTheirTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.WithTestTenantRuntime(t, context.Background())
	own := newClosingDayTraeger(t, db, "Träger Schließtag RLS", 1)
	foreign := newClosingDayTraeger(t, db, "Träger Schließtag RLS Fremd", 1)
	days := schoolCalendarCompose.NewTraegerClosingDays(fixedTraegerSchools{own.id: own.states(), foreign.id: foreign.states()})
	bridge := calendar.NewDate(2027, 5, 7)
	planClosingDay(t, ctx, days, own, bridge, "Brückentag", own.schools[0])
	other := planClosingDay(t, ctx, days, foreign, bridge, "Betriebsausflug", foreign.schools[0])

	require.NoError(t, testpkg.WithinTraeger(ctx, own.id, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		view, err := days.Calendar(ctx, own.id, calendar.NewDate(2027, 1, 1), calendar.NewDate(2027, 12, 31))
		require.NoError(t, err)
		require.Len(t, view.ClosingDays, 1, "only the Träger's own closing day")
		assert.Equal(t, "Brückentag", view.ClosingDays[0].Reason)
		assert.Equal(t, []int64{own.schools[0]}, view.ClosingDays[0].SchoolIDs)
		require.Len(t, view.Holidays, 1, "the holidays of the schools' one federal state")
		assert.Equal(t, "DE-BW", view.Holidays[0].Region.Code)

		_, err = days.UpdateClosingDay(ctx, own.id, other.ID, traegercalendar.ClosingDayInput{
			StartDate: bridge, EndDate: bridge, Reason: "Übernommen", SchoolIDs: own.schools,
		})
		require.ErrorIs(t, err, traegercalendar.ErrClosingDayNotFound, "a foreign closing day is not changed")
		require.ErrorIs(t, days.DeleteClosingDay(ctx, own.id, other.ID), traegercalendar.ErrClosingDayNotFound, "nor deleted")
		return nil
	}))
	require.NoError(t, testpkg.WithinTraeger(ctx, foreign.id, func(ctx context.Context) error {
		view, err := days.Calendar(ctx, foreign.id, calendar.NewDate(2027, 1, 1), calendar.NewDate(2027, 12, 31))
		require.NoError(t, err)
		require.Len(t, view.ClosingDays, 1)
		assert.Equal(t, "Betriebsausflug", view.ClosingDays[0].Reason, "the other Träger keeps its closing day unchanged")
		return nil
	}))
}

// A school outside the Träger is refused before anything is written, so the
// Träger transaction stays usable.
func TestTraegerClosingDayRefusesAForeignSchoolBeforeWriting(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.WithTestTenantRuntime(t, context.Background())
	own := newClosingDayTraeger(t, db, "Träger Schließtag Schule", 1)
	foreign := newClosingDayTraeger(t, db, "Träger Schließtag Schule Fremd", 1)
	days := schoolCalendarCompose.NewTraegerClosingDays(fixedTraegerSchools{own.id: own.states()})
	day := calendar.NewDate(2027, 5, 7)

	require.NoError(t, testpkg.WithinTraeger(ctx, own.id, func(ctx context.Context) error {
		_, err := days.CreateClosingDay(ctx, own.id, traegercalendar.ClosingDayInput{StartDate: day, EndDate: day, Reason: "Fremd", SchoolIDs: foreign.schools})
		require.ErrorIs(t, err, traegercalendar.ErrSchoolOutside)
		_, err = days.CreateClosingDay(ctx, own.id, traegercalendar.ClosingDayInput{StartDate: day, EndDate: day.AddDays(-1), Reason: "Rückwärts", SchoolIDs: own.schools})
		require.ErrorIs(t, err, traegercalendar.ErrInvalidClosingDay)
		_, err = days.CreateClosingDay(ctx, own.id, traegercalendar.ClosingDayInput{StartDate: day, EndDate: day, Reason: "Gültig", SchoolIDs: own.schools})
		require.NoError(t, err, "the transaction is still usable after the refusals")
		return nil
	}))
}

// A school reads the closing days its Träger planned for it as its own, and
// only those: its own filter holds without row level security (rule 25).
// They reach every reader of the school's closing days (Soll, Dienstplan,
// Regeltermine, statistics) and the school cannot change them. Removing the
// school filter of the Träger read turns this test red.
func TestSchoolReadsOnlyTheTraegerClosingDaysNamingItBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.WithTestTenantRuntime(t, context.Background())
	traeger := newClosingDayTraeger(t, db, "Träger Schließtag Schulsicht", 2)
	named, unnamed := traeger.schools[0], traeger.schools[1]
	days := schoolCalendarCompose.NewTraegerClosingDays(fixedTraegerSchools{traeger.id: traeger.states()})
	planClosingDay(t, ctx, days, traeger, calendar.NewDate(2027, 5, 7), "Brückentag", named)
	planClosingDay(t, ctx, days, traeger, calendar.NewDate(2027, 5, 21), "Betriebsausflug", unnamed)
	records, err := schoolCalendarCompose.NewRecords(schoolCalendarCompose.RecordsDependencies{ClosingDayReads: owners.NewSchoolClosingDayReads(), DB: db, Observe: func(schoolCalendarCompose.Observation) {},
		FederalState: func(context.Context) (string, error) { return "DE-NW", nil }})
	require.NoError(t, err)

	schoolCtx := testpkg.ContextForTenant(ctx, named)
	_, err = records.CreateClosingDay(schoolCtx, schoolcalendar.CreateClosingDay{ClosingDayFields: schoolcalendar.ClosingDayFields{
		StartDate: "2027-05-10", EndDate: "2027-05-10", Reason: "Pädagogischer Tag",
	}})
	require.NoError(t, err)
	require.NoError(t, testpkg.WithinCurrentTenant(schoolCtx, func(txCtx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(txCtx))
		list, err := records.ListClosingDays(txCtx, schoolcalendar.ClosingDayFilter{OverlappingFrom: "2027-05-01", OverlappingTo: "2027-05-31"})
		require.NoError(t, err)
		require.Len(t, list, 2, "the school's own closing day and the Träger's for it, not the one for the other school")
		assert.Equal(t, "Brückentag", list[0].Reason)
		assert.True(t, list[0].Traeger, "the Träger's closing day is marked")
		assert.Equal(t, "Pädagogischer Tag", list[1].Reason)
		assert.False(t, list[1].Traeger)

		dates, err := records.ClosingDayDates(txCtx, "2027-05-01", "2027-05-31")
		require.NoError(t, err)
		assert.True(t, dates["2027-05-07"], "the Träger's closing day takes the Soll like the school's own")
		assert.False(t, dates["2027-05-21"], "the other school's closing day does not")
		closing, _, err := records.CareCalendarDays(txCtx, calendar.NewDate(2027, 5, 1), calendar.NewDate(2027, 5, 31))
		require.NoError(t, err)
		assert.True(t, closing[calendar.NewDate(2027, 5, 7)], "the statistics count it as a closing day")
		assert.False(t, closing[calendar.NewDate(2027, 5, 21)])

		byID, err := records.ListClosingDays(txCtx, schoolcalendar.ClosingDayFilter{IDs: []int64{list[0].ID, list[1].ID}})
		require.NoError(t, err)
		for _, day := range byID {
			assert.False(t, day.Traeger, "a read by ID names only the school's own, changeable rows")
		}
		return nil
	}))
	foreign := newClosingDayTraeger(t, db, "Träger nach Schulwechsel", 0)
	_, err = db.ExecContext(ctx, `UPDATE platform.schools SET organization_id = ? WHERE id = ?`, foreign.id, named)
	require.NoError(t, err)
	require.NoError(t, testpkg.WithinCurrentTenant(schoolCtx, func(txCtx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(txCtx))
		list, err := records.ListClosingDays(txCtx, schoolcalendar.ClosingDayFilter{})
		require.NoError(t, err)
		require.Len(t, list, 1, "after transfer only the school's own closing day remains")
		assert.False(t, list[0].Traeger)
		dates, err := records.ClosingDayDates(txCtx, "2027-05-01", "2027-05-31")
		require.NoError(t, err)
		assert.False(t, dates["2027-05-07"], "the former Träger no longer changes the school's Soll")
		closing, _, err := records.CareCalendarDays(txCtx, calendar.NewDate(2027, 5, 1), calendar.NewDate(2027, 5, 31))
		require.NoError(t, err)
		assert.False(t, closing[calendar.NewDate(2027, 5, 7)], "nor its care calendar")
		return nil
	}))

}
