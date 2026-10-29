package timetablehttp

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// draftRosterParam writes one `roster` value: the children planned on a
// weekday of a Regeltermin that is still being planned (#5104).
func draftRosterParam(weekday int, studentIDs []int64) string {
	ids := make([]string, 0, len(studentIDs))
	for _, id := range studentIDs {
		ids = append(ids, fmt.Sprint(id))
	}
	return fmt.Sprintf("%d:%s", weekday, strings.Join(ids, ","))
}

func (s *rosterReviewSetup) draftRouter() chi.Router {
	s.router.Get("/offering-sources/suggestion", s.res.suggestDraftOfferingSource)
	s.router.Get("/offering-sources/preview", s.res.previewDraftOfferingSource)
	return s.router
}

func getDraftSuggestion(t *testing.T, router http.Handler, query url.Values) *offeringSuggestionResponse {
	t.Helper()
	w := doTemplateJSON(t, router, http.MethodGet, "/offering-sources/suggestion?"+query.Encode(), nil)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	return decodeTemplateData[offeringSuggestionEnvelope](t, w).Suggestion
}

// TestDraftOfferingSuggestion: a Klasse Regeltermin on Monday and Wednesday
// with the whole class 1b (#5104). Randstunde books four of the six children
// on both days, so the dialog suggests it with the Klassenfilter. An exact
// match still suggests; no bookings, fewer than three children or a booking
// below the rule suggest nothing.
func TestDraftOfferingSuggestion(t *testing.T) {
	t.Parallel()
	sources := &rosterMaintenanceSources{}
	s := buildRosterReviewModule(t, sources)
	router := s.draftRouter()
	period := createTemplateTestPeriodRange(t, s.db, "Draft-Period", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	class := createClassStudents(t, s.db, "1b", 6)
	sources.bookings = rosterReviewBookings(rosterReviewOfferingID, "Randstunde", "1b", class[:4], 1, 3)
	query := func(students []int64) url.Values {
		return url.Values{
			"period_id":      {fmt.Sprint(period.ID)},
			"type":           {timetableModule.GroupTypeCare},
			"list_kind":      {"edge_hours"},
			"school_classes": {"1b"},
			"roster":         {draftRosterParam(1, students), draftRosterParam(3, students)},
		}
	}

	suggestion := getDraftSuggestion(t, router, query(class))
	require.NotNil(t, suggestion)
	assert.Equal(t, templateRosterMaintenanceOfferingResponse{ID: rosterReviewOfferingID, Name: "Randstunde"}, suggestion.Offering)
	assert.Equal(t, []string{"1b"}, suggestion.SchoolClasses)
	assert.Empty(t, suggestion.GradeLevels)
	assert.Equal(t, 2, suggestion.UnbookedCount)
	assert.Zero(t, suggestion.MissingCount)
	assert.False(t, suggestion.Ambiguous)

	exact := getDraftSuggestion(t, router, query(class[:4]))
	require.NotNil(t, exact, "a new Regeltermin that matches the bookings exactly is still suggested")
	assert.Zero(t, exact.UnbookedCount)

	assert.Nil(t, getDraftSuggestion(t, router, query(class[:2])), "fewer than three children")

	aboveRule := query(class)
	aboveRule.Set("type", timetableModule.GroupTypeActivity)
	assert.Nil(t, getDraftSuggestion(t, router, aboveRule), "an AG is not explained by a care offering")

	sources.bookings = nil
	assert.Nil(t, getDraftSuggestion(t, router, query(class)), "no bookings, for example with care offerings off")
}

// TestDraftOfferingSuggestionAmbiguous (#5138): Randstunde and Ganztag both
// book only children of class 1b, so both cover the class fully. The answer
// says so and lists both; the dialog then preselects neither.
func TestDraftOfferingSuggestionAmbiguous(t *testing.T) {
	t.Parallel()
	sources := &rosterMaintenanceSources{}
	s := buildRosterReviewModule(t, sources)
	router := s.draftRouter()
	period := createTemplateTestPeriodRange(t, s.db, "Draft-Period", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	class := createClassStudents(t, s.db, "1b", 6)
	ganztagID := rosterReviewOfferingID + 1
	sources.bookings = append(
		rosterReviewBookings(rosterReviewOfferingID, "Randstunde", "1b", class[:3], 1, 3),
		rosterReviewBookings(ganztagID, "Ganztag", "1b", class[1:5], 1, 3)...,
	)

	suggestion := getDraftSuggestion(t, router, url.Values{
		"period_id":      {fmt.Sprint(period.ID)},
		"type":           {timetableModule.GroupTypeCare},
		"list_kind":      {"edge_hours"},
		"school_classes": {"1b"},
		"roster":         {draftRosterParam(1, class), draftRosterParam(3, class)},
	})

	require.NotNil(t, suggestion)
	assert.True(t, suggestion.Ambiguous)
	assert.Equal(t, []templateRosterMaintenanceOfferingResponse{
		{ID: ganztagID, Name: "Ganztag"},
		{ID: rosterReviewOfferingID, Name: "Randstunde"},
	}, suggestion.Candidates)
}

// TestDraftOfferingPreview: switching the planned Regeltermin to Randstunde
// shows per weekday who joins and who leaves, with names.
func TestDraftOfferingPreview(t *testing.T) {
	t.Parallel()
	sources := &rosterMaintenanceSources{}
	s := buildRosterReviewModule(t, sources)
	router := s.draftRouter()
	period := createTemplateTestPeriodRange(t, s.db, "Draft-Preview", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	class := createClassStudents(t, s.db, "1b", 5)
	sources.bookings = rosterReviewBookings(rosterReviewOfferingID, "Randstunde", "1b", class[1:5], 1)
	query := url.Values{
		"period_id":      {fmt.Sprint(period.ID)},
		"offering_ids":   {fmt.Sprint(rosterReviewOfferingID)},
		"school_classes": {"1b"},
		"roster":         {draftRosterParam(1, class[:4]), draftRosterParam(3, class[:2])},
		"from":           {rosterReviewMonday.String()},
	}

	w := doTemplateJSON(t, router, http.MethodGet, "/offering-sources/preview?"+query.Encode(), nil)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	preview := decodeTemplateData[offeringPreviewResponse](t, w)

	assert.Equal(t, rosterReviewMonday.String(), preview.From)
	assert.Empty(t, preview.Segments)
	require.Len(t, preview.Weekdays, 2)
	monday, wednesday := preview.Weekdays[0], preview.Weekdays[1]
	assert.Equal(t, 1, monday.Weekday)
	require.Len(t, monday.Added, 1)
	assert.Equal(t, class[4], monday.Added[0].StudentID)
	assert.NotEmpty(t, monday.Added[0].Name)
	require.Len(t, monday.Removed, 1)
	assert.Equal(t, class[0], monday.Removed[0].StudentID)
	assert.Equal(t, 3, monday.KeptCount)
	assert.Equal(t, 3, wednesday.Weekday)
	assert.Len(t, wednesday.Removed, 2, "nobody books Wednesday")
	assert.Empty(t, wednesday.Added)
}

// TestDraftOfferingSourceRejectsBadInput: a malformed roster, a weekday
// outside the week, a missing period or a past from are refused.
func TestDraftOfferingSourceRejectsBadInput(t *testing.T) {
	t.Parallel()
	sources := &rosterMaintenanceSources{}
	s := buildRosterReviewModule(t, sources)
	router := s.draftRouter()
	period := createTemplateTestPeriodRange(t, s.db, "Draft-Refusals", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	base := func() url.Values {
		return url.Values{
			"period_id":    {fmt.Sprint(period.ID)},
			"type":         {timetableModule.GroupTypeCare},
			"offering_ids": {fmt.Sprint(rosterReviewOfferingID)},
			"roster":       {"1:11,12,13"},
		}
	}
	cases := map[string]func(url.Values) url.Values{
		"roster without weekday": func(q url.Values) url.Values { q.Set("roster", "11,12"); return q },
		"weekday eight":          func(q url.Values) url.Values { q.Set("roster", "8:11"); return q },
		"negative child":         func(q url.Values) url.Values { q.Set("roster", "1:-4"); return q },
		"no period":              func(q url.Values) url.Values { q.Del("period_id"); return q },
		"too many children": func(q url.Values) url.Values {
			ids := make([]int64, 0, maxDraftRosterChildren+1)
			for i := range maxDraftRosterChildren + 1 {
				ids = append(ids, int64(i+1))
			}
			q.Set("roster", draftRosterParam(1, ids))
			return q
		},
	}
	for name, mutate := range cases {
		for _, path := range []string{"/offering-sources/suggestion", "/offering-sources/preview"} {
			w := doTemplateJSON(t, router, http.MethodGet, path+"?"+mutate(base()).Encode(), nil)
			assert.Equal(t, http.StatusBadRequest, w.Code, "%s %s: %s", path, name, w.Body.String())
		}
	}
	past := base()
	past.Set("from", rosterReviewMonday.AddDays(-1).String())
	w := doTemplateJSON(t, router, http.MethodGet, "/offering-sources/preview?"+past.Encode(), nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

// TestDraftOfferingSourceQueryBudget keeps both reads flat in the number of
// planned children: one bookings read (stubbed here; Care Plan's read has
// its own budget) and, for the preview, two name reads.
func TestDraftOfferingSourceQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	sources := &rosterMaintenanceSources{}
	s := buildRosterReviewModule(t, sources)
	period := createTemplateTestPeriodRange(t, s.db, "Draft-Budget", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	class := createClassStudents(t, s.db, "4c", 8)
	counter := testpkg.CaptureQueries(t, s.db)
	measured := chi.NewRouter()
	measured.Use(render.SetContentType(render.ContentTypeJSON))
	measured.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(testpkg.WithTestTenantRuntime(t, r.Context())))
		})
	})
	measured.Get("/offering-sources/suggestion", s.res.suggestDraftOfferingSource)
	measured.Get("/offering-sources/preview", s.res.previewDraftOfferingSource)
	router := testpkg.MeasuredTenantRequest(t, s.db, testpkg.TenantIDFromContext(s.ctx), counter, measured)

	run := func(path string, children []int64) int {
		sources.bookings = rosterReviewBookings(rosterReviewOfferingID, "Randstunde", "4c", children, 1)
		query := url.Values{
			"period_id":    {fmt.Sprint(period.ID)},
			"type":         {timetableModule.GroupTypeCare},
			"offering_ids": {fmt.Sprint(rosterReviewOfferingID)},
			"roster":       {draftRosterParam(1, children), draftRosterParam(3, children)},
			"from":         {rosterReviewMonday.String()},
		}
		counter.Reset()
		w := doTemplateJSON(t, router, http.MethodGet, path+"?"+query.Encode(), nil)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		return counter.Total()
	}

	suggestSmall, suggestLarge := run("/offering-sources/suggestion", class[:3]), run("/offering-sources/suggestion", class)
	assert.Equal(t, suggestSmall, suggestLarge, "the suggestion must not grow with the children")
	testpkg.AssertQueryBudget(t, "api.timetable.offering_sources.suggestion", counter.Queries())

	previewSmall, previewLarge := run("/offering-sources/preview", class[:3]), run("/offering-sources/preview", class)
	assert.Equal(t, previewSmall, previewLarge, "the preview must not grow with the children")
	testpkg.AssertQueryBudget(t, "api.timetable.offering_sources.preview", counter.Queries())
}
