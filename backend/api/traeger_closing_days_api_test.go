package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

type traegerClosingDayBody struct {
	ID        int64   `json:"id"`
	StartDate string  `json:"start_date"`
	EndDate   string  `json:"end_date"`
	Reason    string  `json:"reason"`
	SchoolIDs []int64 `json:"school_ids"`
}

type traegerCalendarBody struct {
	ClosingDays []traegerClosingDayBody `json:"closing_days"`
	Schools     []struct {
		ID           int64 `json:"id"`
		FederalState struct {
			Code string `json:"code"`
			Name string `json:"name"`
		} `json:"federal_state"`
	} `json:"schools"`
	Holidays []struct {
		Region struct {
			Code string `json:"code"`
		} `json:"region"`
		Holidays []struct {
			Date string `json:"date"`
			Name string `json:"name"`
		} `json:"holidays"`
	} `json:"holidays"`
}

const traegerCalendar2027 = "/traeger/closing-days/?from=2027-01-01&to=2027-12-31"

func traegerClosingDayPath(id int64) string {
	return "/traeger/closing-days/" + strconv.FormatInt(id, 10)
}

func traegerCalendarOf(t *testing.T, router http.Handler, office traegerOffice) traegerCalendarBody {
	t.Helper()
	rec := traegerJSON(t, router, office.token, http.MethodGet, traegerCalendar2027, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[traegerCalendarBody](t, rec.Body.Bytes())
}

// The Träger-Büro plans a closing day for some of its schools and sees each
// school's federal state with that state's holidays, read only (#4702).
func TestTraegerPlansClosingDaysAndSeesEachSchoolsFederalState(t *testing.T) {
	t.Parallel()
	db, router := newMasterDataPortal(t)
	office := newTraegerOffice(t, db, router, "Träger Schließtage", identityaccess.TraegerRoleAdministration)

	rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/closing-days/", traegerClosingDayBody{
		StartDate: "2027-05-07", EndDate: "2027-05-07", Reason: "  Brückentag ", SchoolIDs: []int64{office.schools[0]},
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created := decodeSchoolPortalData[traegerClosingDayBody](t, rec.Body.Bytes())
	assert.Equal(t, "Brückentag", created.Reason)

	view := traegerCalendarOf(t, router, office)
	require.Len(t, view.ClosingDays, 1)
	assert.Equal(t, []int64{office.schools[0]}, view.ClosingDays[0].SchoolIDs)
	require.Len(t, view.Schools, 2)
	for _, school := range view.Schools {
		assert.Equal(t, "DE-NW", school.FederalState.Code, "the federal state the moto team set, read only")
		assert.Equal(t, "Nordrhein-Westfalen", school.FederalState.Name)
	}
	require.Len(t, view.Holidays, 1)
	assert.Contains(t, view.Holidays[0].Holidays, struct {
		Date string `json:"date"`
		Name string `json:"name"`
	}{Date: "2027-05-27", Name: "Fronleichnam"})

	created.SchoolIDs = office.schools
	created.EndDate = "2027-05-08"
	rec = traegerJSON(t, router, office.token, http.MethodPut, traegerClosingDayPath(created.ID), created)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.ElementsMatch(t, office.schools, traegerCalendarOf(t, router, office).ClosingDays[0].SchoolIDs)

	rec = traegerJSON(t, router, office.token, http.MethodPost, "/traeger/closing-days/", traegerClosingDayBody{
		StartDate: "2027-05-07", EndDate: "2027-05-06", Reason: "Rückwärts", SchoolIDs: office.schools,
	})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "traeger.closing_day_invalid", refusalOf(t, rec).Code)

	rec = traegerJSON(t, router, office.token, http.MethodDelete, traegerClosingDayPath(created.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, traegerCalendarOf(t, router, office).ClosingDays)
}

// Planning needs the right "Einstellungen verwalten"; every Träger role reads
// the calendar of its own Träger only, and a Träger reaches neither the
// closing days nor the schools of another.
func TestTraegerClosingDaysNeedTheSettingsRightAndStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	db, router := newMasterDataPortal(t)
	own := newTraegerOffice(t, db, router, "Träger Schließtage Eigen", identityaccess.TraegerRoleAdministration)
	other := newTraegerOffice(t, db, router, "Träger Schließtage Fremd", identityaccess.TraegerRoleAdministration)
	rec := traegerJSON(t, router, own.token, http.MethodPost, "/traeger/closing-days/", traegerClosingDayBody{
		StartDate: "2027-05-07", EndDate: "2027-05-07", Reason: "Brückentag", SchoolIDs: own.schools,
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	day := decodeSchoolPortalData[traegerClosingDayBody](t, rec.Body.Bytes())

	reader := newTraegerOffice(t, db, router, "Träger Schließtage Lesend", identityaccess.TraegerRoleReadOnly)
	assert.Empty(t, traegerCalendarOf(t, router, reader).ClosingDays, "a reader sees its own Träger only")
	for i, write := range []*httptest.ResponseRecorder{
		traegerJSON(t, router, reader.token, http.MethodPost, "/traeger/closing-days/", traegerClosingDayBody{StartDate: "2027-05-07", EndDate: "2027-05-07", Reason: "X", SchoolIDs: reader.schools}),
		traegerJSON(t, router, reader.token, http.MethodPut, traegerClosingDayPath(day.ID), day),
		traegerJSON(t, router, reader.token, http.MethodDelete, traegerClosingDayPath(day.ID), nil),
	} {
		assert.Equal(t, http.StatusForbidden, write.Code, "write %d: %s", i, write.Body.String())
	}

	view := traegerCalendarOf(t, router, other)
	assert.Empty(t, view.ClosingDays)
	for _, school := range view.Schools {
		assert.NotContains(t, own.schools, school.ID, "another Träger's schools stay hidden")
	}
	rec = traegerJSON(t, router, other.token, http.MethodPost, "/traeger/closing-days/", traegerClosingDayBody{
		StartDate: "2027-05-07", EndDate: "2027-05-07", Reason: "Fremd", SchoolIDs: own.schools,
	})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "traeger.closing_day_school_outside", refusalOf(t, rec).Code)
	day.Reason = "Übernommen"
	rec = traegerJSON(t, router, other.token, http.MethodPut, traegerClosingDayPath(day.ID), traegerClosingDayBody{
		StartDate: day.StartDate, EndDate: day.EndDate, Reason: day.Reason, SchoolIDs: other.schools,
	})
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Equal(t, "traeger.closing_day_not_found", refusalOf(t, rec).Code)
	rec = traegerJSON(t, router, other.token, http.MethodDelete, traegerClosingDayPath(day.ID), nil)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	kept := traegerCalendarOf(t, router, own).ClosingDays
	require.Len(t, kept, 1)
	assert.Equal(t, "Brückentag", kept[0].Reason, "the other Träger changed nothing")
}

// The calendar of the Träger reads the same statements for any number of
// closing days.
func TestTraegerClosingDaysQueryBudget(t *testing.T) {
	t.Parallel()
	db, router := newMasterDataPortal(t)
	office := newTraegerOffice(t, db, router, "Träger Budget Schließtage", identityaccess.TraegerRoleAdministration)
	plan := func(day string) {
		rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/closing-days/", traegerClosingDayBody{
			StartDate: day, EndDate: day, Reason: "Schließtag", SchoolIDs: office.schools,
		})
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	}
	plan("2027-05-07")
	read := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, db)
		req := httptest.NewRequest(http.MethodGet, traegerCalendar2027, nil)
		req.Header.Set("Authorization", "Bearer "+office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, router, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	one := read()
	plan("2027-10-04")
	plan("2027-12-24")
	assert.Len(t, read().Statements, len(one.Statements), "the calendar reads the same statements for any number of closing days")
	querybudgets.AssertQueryBudget(t, "api.traeger.closing_days", one)
}
