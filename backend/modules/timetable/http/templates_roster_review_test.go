package timetablehttp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// The roster review suites (#5103) run on a fixed Monday far ahead of any
// real clock, so the review's own today and the reconcile's wall clock agree
// that every arranged occurrence lies in the future.
var rosterReviewMonday = calendar.NewDate(2030, 3, 4)

func rosterReviewClock() time.Time {
	return rosterReviewMonday.BerlinMidnight().Add(7 * time.Hour)
}

// rosterReviewOfferingID stands for a Betreuungsangebot of the stubbed
// offering side; the timetable side only stores the id.
const rosterReviewOfferingID = int64(880001)

type rosterReviewSetup struct {
	*templateSetup
	router  chi.Router
	resyncs []timetableModule.OfferingRosterResyncInput
	// booked are the children the fake resync plans as offering-sourced.
	booked []int64
}

// buildRosterReviewModule composes the real template writes and roster
// review over the test database. Care Plan is replaced: the offering checks
// accept, and the resync writes one offering-sourced row per booked child,
// as Care Plan's resync would.
func buildRosterReviewModule(t *testing.T, sources timetableModule.OfferingSourceSupport) *rosterReviewSetup {
	t.Helper()
	mat := &mockMaterializationService{result: &timetableModule.MaterializationResult{}}
	s := buildTemplateModule(t, mat, rosterReviewClock)
	setup := &rosterReviewSetup{templateSetup: s}
	composed := testTimetableWith(s.db, testTimetableOptions{
		validateCareOfferingSeries: acceptCareOfferingSeries,
		validateOfferingSource:     func(context.Context, []int64, []int64, *int64) error { return nil },
		resyncOfferingRoster:       setup.fakeResync,
		materialization:            mat,
		instances:                  s.res.InstanceService,
	}, rosterReviewClock)
	s.res.Templates = composed
	s.res.RosterReview = composed.RosterReview()
	s.res.OfferingSourceOptions = sources
	s.res.People = must(RootCompositions.People(s.db))
	setup.router = rosterReviewRouter(s.ctx, s.res, s.db)
	return setup
}

func (s *rosterReviewSetup) fakeResync(ctx context.Context, in timetableModule.OfferingRosterResyncInput) error {
	s.resyncs = append(s.resyncs, in)
	for _, studentID := range s.booked {
		if _, err := s.owner.CreateStudentEnrollment(ctx, timetableModule.StudentEnrollmentInput{
			StudentID: studentID, ActivityGroupID: in.TemplateID, ValidFrom: in.EffectiveFrom.String(),
			CalendarPeriodID: in.CalendarPeriodID, SelectedWeekdays: []int{1, 3},
		}); err != nil {
			return err
		}
	}
	return nil
}

func rosterReviewRouter(parentCtx context.Context, res *Resource, db *bun.DB) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(req.Context()), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Use(testpkg.TenantTxMiddleware(db))
	r.Get("/templates", res.listTemplates)
	r.Get("/templates/roster-deviations", res.listTemplateRosterDeviations)
	r.Get("/templates/{id}", res.getTemplate)
	r.Post("/templates", res.createTemplate)
	r.Post("/templates/{id}/split", res.splitTemplate)
	r.Get("/templates/{id}/offering-preview", res.previewTemplateOfferingSource)
	r.Post("/templates/{id}/offering-connection", res.connectTemplateOfferingSource)
	r.Put("/templates/{id}/manual-roster", res.setTemplateManualRoster)
	return r
}

// createClassTemplate creates a Klasse Regeltermin on Monday and Wednesday
// with the children picked by hand, the Randstunden blocks of Schule am Berg.
func (s *rosterReviewSetup) createClassTemplate(t *testing.T, name, class string, periodID int64, studentIDs []int64) int64 {
	t.Helper()
	body := map[string]any{
		"name":                name,
		"type":                timetableModule.GroupTypeCare,
		"weekdays":            []int{timetableModule.WeekdayMonday, timetableModule.WeekdayWednesday},
		"start_time":          "07:00",
		"end_time":            "08:00",
		"room_id":             s.roomID,
		"category_id":         s.category.ID,
		"calendar_period_id":  periodID,
		"target_group_type":   timetableModule.TargetGroupTypeSchoolClass,
		"target_school_class": class,
		"student_ids":         studentIDs,
		"staff_ids":           []int64{s.staffA},
	}
	w := doTemplateJSON(t, s.router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	return decodeTemplateData[createTemplateResponse](t, w).TemplateID
}

func createClassStudents(t *testing.T, db *bun.DB, class string, count int) []int64 {
	t.Helper()
	ids := make([]int64, 0, count)
	for i := range count {
		student := testpkg.CreateTestStudent(t, db, "Kind", fmt.Sprintf("Review-%s-%d-%d", class, i, time.Now().UnixNano()), class)
		ids = append(ids, student.ID)
	}
	return ids
}

// rosterReviewBookings books care offerings: the Regeltermine of these tests
// are care blocks, which only care offerings can explain.
func rosterReviewBookings(offeringID int64, name, class string, studentIDs []int64, weekdays ...int) []timetableModule.OfferingWeekdayBooking {
	bookings := make([]timetableModule.OfferingWeekdayBooking, 0, len(studentIDs))
	for _, studentID := range studentIDs {
		bookings = append(bookings, timetableModule.OfferingWeekdayBooking{
			OfferingID: offeringID, OfferingName: name, StudentID: studentID, SchoolClass: class, Weekdays: weekdays,
			CountsAsCare: true,
		})
	}
	return bookings
}

// TestTemplateRosterDeviationsSchuleAmBerg: the whole class 1a sits in a
// Klasse Regeltermin by hand; "Randstunde" books four of its six children on
// Monday and Wednesday, "Ganztag" one child. The hint names Randstunde and,
// for that child, Ganztag (#5139) with the Klassenfilter and counts the one
// child without a booking. An
// automatic Regeltermin, a roster with too few booked children and a series
// confirmed as hand-kept show no hint.
func TestTemplateRosterDeviationsSchuleAmBerg(t *testing.T) {
	t.Parallel()
	sources := &rosterMaintenanceSources{results: map[int64]timetableModule.TemplateRosterMaintenance{}}
	s := buildRosterReviewModule(t, sources)
	period := createTemplateTestPeriodRange(t, s.db, "Review-Period", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	class := createClassStudents(t, s.db, "1a", 6)
	randstunde := rosterReviewOfferingID
	sources.bookings = append(rosterReviewBookings(randstunde, "Randstunde", "1a", class[:4], 1, 3),
		rosterReviewBookings(randstunde+1, "Ganztag", "1a", class[4:5], 1, 3, 5)...)

	deviating := s.createClassTemplate(t, "RDST 1a", "1a", period.ID, class)
	automatic := s.createClassTemplate(t, "RDST 1a automatisch", "1a", period.ID, class)
	fewBooked := s.createClassTemplate(t, "RDST 1a wenig", "1a", period.ID, class[:2])
	confirmed := s.createClassTemplate(t, "RDST 1a bewusst", "1a", period.ID, class)
	for _, id := range []int64{deviating, fewBooked, confirmed} {
		sources.results[id] = timetableModule.TemplateRosterMaintenance{Mode: "manual", DynamicTargetsManual: true}
	}
	sources.results[automatic] = timetableModule.TemplateRosterMaintenance{Mode: "automatic"}
	// fewBooked plans only class[:2]; the Klasse target adds the whole class
	// at materialization, so drop it to keep the two hand-picked children.
	replaceTemplateTargets(t, s.templateSetup, fewBooked)

	w := doTemplateJSON(t, s.router, http.MethodPut, fmt.Sprintf("/templates/%d/manual-roster", confirmed), map[string]any{"confirmed": true})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.True(t, decodeTemplateData[manualRosterResponse](t, w).ManualRosterConfirmed)

	// A caller without the right to read children gets the counts only.
	for _, deviation := range s.listDeviations(t, period.ID) {
		assert.Empty(t, deviation.Unbooked, "names need the right to read children")
		assert.Empty(t, deviation.Missing)
	}
	deviations := s.listDeviationsAs(t, period.ID, "*:*")

	require.Len(t, deviations, 1, "only the hand-kept Klasse Regeltermin deviates: %+v", deviations)
	got := deviations[0]
	assert.Equal(t, deviating, got.TemplateID)
	assert.Equal(t, []templateRosterMaintenanceOfferingResponse{
		{ID: randstunde, Name: "Randstunde"}, {ID: randstunde + 1, Name: "Ganztag"},
	}, got.Offerings)
	assert.Equal(t, []string{"1a"}, got.SchoolClasses)
	assert.Empty(t, got.GradeLevels)
	assert.Equal(t, 1, got.UnbookedCount, "class[4] books Ganztag; only class[5] books nothing")
	assert.Zero(t, got.MissingCount)
	// The hint names that child and its weekdays.
	require.Len(t, got.Unbooked, 1)
	assert.Equal(t, class[5], got.Unbooked[0].StudentID)
	assert.Contains(t, got.Unbooked[0].Name, "Review-1a-5-")
	assert.Equal(t, []int{timetableModule.WeekdayMonday, timetableModule.WeekdayWednesday}, got.Unbooked[0].Weekdays)
	assert.Empty(t, got.Missing)
	assert.Equal(t, got.Offerings, got.Candidates, "no other offering passes the rule")

	// „Mit Angebot verbinden“ from a booking change names one series: it is
	// reviewed although it is kept by hand, so the link reaches the dialog.
	// The rest of the period stays in the answer for the cards.
	named := func(templateID int64) []int64 {
		w := doTemplateJSON(t, s.router, http.MethodGet, fmt.Sprintf("/templates/roster-deviations?period_id=%d&template_id=%d", period.ID, templateID), nil)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		ids := []int64{}
		for _, deviation := range decodeTemplateData[rosterDeviationsResponse](t, w).Deviations {
			ids = append(ids, deviation.TemplateID)
		}
		slices.Sort(ids)
		return ids
	}
	assert.Equal(t, []int64{deviating, confirmed}, named(confirmed), "the named hand-kept series is reviewed")
	assert.Equal(t, []int64{deviating}, named(automatic), "naming an automatic series reviews nothing more")
	w = doTemplateJSON(t, s.router, http.MethodGet, fmt.Sprintf("/templates/roster-deviations?period_id=%d&template_id=x", period.ID), nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// The list carries the decision; undoing it brings the hint back.
	list := decodeTemplateData[listTemplatesResponse](t,
		doTemplateJSON(t, s.router, http.MethodGet, fmt.Sprintf("/templates?period_id=%d", period.ID), nil)).Templates
	for _, template := range list {
		assert.Equal(t, template.ID == confirmed, template.ManualRosterConfirmed, "template %d", template.ID)
	}
	w = doTemplateJSON(t, s.router, http.MethodPut, fmt.Sprintf("/templates/%d/manual-roster", confirmed), map[string]any{"confirmed": false})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	ids := []int64{}
	for _, deviation := range s.listDeviations(t, period.ID) {
		ids = append(ids, deviation.TemplateID)
	}
	slices.Sort(ids)
	assert.Equal(t, []int64{deviating, confirmed}, ids)
}

// listDeviationsAs reads the hints as a caller with these permissions.
func (s *rosterReviewSetup) listDeviationsAs(t *testing.T, periodID int64, granted ...string) []rosterDeviationResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/templates/roster-deviations?period_id=%d", periodID), nil)
	req = req.WithContext(testpkg.WithPermissionsContext(req.Context(), granted))
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	return decodeTemplateData[rosterDeviationsResponse](t, w).Deviations
}

func (s *rosterReviewSetup) listDeviations(t *testing.T, periodID int64) []rosterDeviationResponse {
	t.Helper()
	w := doTemplateJSON(t, s.router, http.MethodGet, fmt.Sprintf("/templates/roster-deviations?period_id=%d", periodID), nil)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	return decodeTemplateData[rosterDeviationsResponse](t, w).Deviations
}

func TestTemplateManualRosterRejectsUnknownTemplate(t *testing.T) {
	t.Parallel()
	s := buildRosterReviewModule(t, &rosterMaintenanceSources{})
	w := doTemplateJSON(t, s.router, http.MethodPut, "/templates/999999/manual-roster", map[string]any{"confirmed": true})
	assert.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
	w = doTemplateJSON(t, s.router, http.MethodPut, "/templates/999999/manual-roster", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

// seriesFixture is a Klasse Regeltermin split into two segments: A runs up to
// the split (exclusive), B from it on, each with planned occurrences holding
// the whole hand roster.
type seriesFixture struct {
	periodID    int64
	segmentA    int64
	segmentB    int64
	split       calendar.Date
	occurrences map[int64][]int64 // segment → instance ids
	booked      []int64
	unbooked    []int64
}

func (s *rosterReviewSetup) arrangeTwoSegmentSeries(t *testing.T) seriesFixture {
	t.Helper()
	period := testpkg.CreateTestCalendarPeriod(t, s.db, fmt.Sprintf("Review-Series-%d", time.Now().UnixNano()),
		rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120))
	class := createClassStudents(t, s.db, "2c", 4)
	fixture := seriesFixture{periodID: period.ID, split: rosterReviewMonday.AddDays(14), booked: class[:2], unbooked: class[2:]}
	fixture.segmentA = s.createClassTemplate(t, "RDST 2c", "2c", period.ID, class)

	splitRequest := map[string]any{
		"name": "RDST 2c", "type": timetableModule.GroupTypeCare,
		"weekdays":   []int{timetableModule.WeekdayMonday, timetableModule.WeekdayWednesday},
		"start_time": "07:00", "end_time": "08:00", "room_id": s.roomID, "category_id": s.category.ID,
		"calendar_period_id": period.ID, "target_group_type": timetableModule.TargetGroupTypeSchoolClass,
		"target_school_class": "2c", "student_ids": class, "staff_ids": []int64{s.staffA},
		"effective_date": fixture.split.String(),
	}
	w := doTemplateJSON(t, s.router, http.MethodPost, fmt.Sprintf("/templates/%d/split", fixture.segmentA), splitRequest)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	fixture.segmentB = decodeTemplateData[splitTemplateResponse](t, w).NewTemplateID

	fixture.occurrences = map[int64][]int64{}
	dates := map[int64][]calendar.Date{
		fixture.segmentA: {rosterReviewMonday.AddDays(2), rosterReviewMonday.AddDays(7)},
		fixture.segmentB: {fixture.split, fixture.split.AddDays(2)},
	}
	for segment, segmentDates := range dates {
		for _, date := range segmentDates {
			instance := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
				ActivityGroupID: &segment, CalendarPeriodID: &period.ID, StartHHMM: "07:00", EndHHMM: "08:00",
			})
			for _, studentID := range class {
				testpkg.CreateTestInstanceStudent(t, s.db, instance.ID, studentID, "")
			}
			fixture.occurrences[segment] = append(fixture.occurrences[segment], instance.ID)
		}
	}
	return fixture
}

func (s *rosterReviewSetup) occurrenceStudents(t *testing.T, instanceID int64) []int64 {
	t.Helper()
	var ids []int64
	require.NoError(t, s.db.NewSelect().TableExpr("schedule.instance_students").Column("student_id").
		Where("instance_id = ?", instanceID).Order("student_id").Scan(s.ctx, &ids))
	return ids
}

// TestTemplateOfferingConnectionCoversEveryFutureSegment: Schule am Berg's
// series end one segment and start the next later. Connecting from the next
// occurrence switches both segments to the offering, drops the hand-picked
// children without a booking from every planned occurrence of both, keeps
// the booked ones, and makes the Care Plan resync feed each segment from its
// own boundary.
func TestTemplateOfferingConnectionCoversEveryFutureSegment(t *testing.T) {
	t.Parallel()
	s := buildRosterReviewModule(t, &rosterMaintenanceSources{})
	fixture := s.arrangeTwoSegmentSeries(t)
	s.booked = fixture.booked
	effective := rosterReviewMonday.AddDays(2)

	// The preview names both segments and, per weekday, who leaves.
	w := doTemplateJSON(t, s.router, http.MethodGet, fmt.Sprintf(
		"/templates/%d/offering-preview?offering_ids=%d&school_classes=2c", fixture.segmentB, rosterReviewOfferingID), nil)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	preview := decodeTemplateData[offeringPreviewResponse](t, w)
	assert.Equal(t, effective.String(), preview.From, "the preview starts at the next planned occurrence")
	require.Len(t, preview.Segments, 2)
	assert.Equal(t, fixture.segmentA, preview.Segments[0].TemplateID)
	assert.Equal(t, fixture.segmentB, preview.Segments[1].TemplateID)
	require.NotNil(t, preview.Segments[0].ValidUntil)
	assert.Equal(t, fixture.split.String(), *preview.Segments[0].ValidUntil)
	assert.Nil(t, preview.Segments[1].ValidUntil)

	announcer := &recordingStaffingAnnouncer{}
	s.res.Staffing = announcer
	w = doTemplateJSON(t, s.router, http.MethodPost, fmt.Sprintf("/templates/%d/offering-connection", fixture.segmentB), map[string]any{
		"offering_ids": []int64{rosterReviewOfferingID}, "school_classes": []string{"2c"}, "effective_date": effective.String(),
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	// Open planners and supervision views learn about the rewritten rosters
	// after the commit (#5017).
	assert.Equal(t, []string{"template_offering_connection"}, announcer.sources)
	result := decodeTemplateData[offeringConnectionResponse](t, w)
	assert.Equal(t, []int64{fixture.segmentA, fixture.segmentB}, result.TemplateIDs)
	assert.Equal(t, effective.String(), result.EffectiveDate)

	require.Len(t, s.resyncs, 2)
	assert.Equal(t, fixture.segmentA, s.resyncs[0].TemplateID)
	assert.Equal(t, effective, s.resyncs[0].EffectiveFrom)
	assert.Equal(t, fixture.segmentB, s.resyncs[1].TemplateID)
	assert.Equal(t, fixture.split, s.resyncs[1].EffectiveFrom)
	for _, resync := range s.resyncs {
		assert.Equal(t, []int64{rosterReviewOfferingID}, resync.OfferingIDs)
		assert.Equal(t, []string{"2c"}, resync.SchoolClasses)
	}

	for _, segment := range []int64{fixture.segmentA, fixture.segmentB} {
		var row struct {
			TargetGroupType   string  `bun:"target_group_type"`
			TargetSchoolClass *string `bun:"target_school_class"`
			Sources           string  `bun:"sources"`
			Targets           int     `bun:"targets"`
		}
		require.NoError(t, s.db.NewRaw(`SELECT g.target_group_type, g.target_school_class,
				COALESCE(g.source_care_offering_ids::text, '') AS sources,
				(SELECT COUNT(*) FROM activities.group_targets gt WHERE gt.activity_group_id = g.id) AS targets
			FROM activities.groups g WHERE g.id = ?`, segment).Scan(s.ctx, &row))
		assert.Equal(t, timetableModule.TargetGroupTypeOffering, row.TargetGroupType, "segment %d", segment)
		assert.Nil(t, row.TargetSchoolClass)
		assert.Equal(t, fmt.Sprintf("[%d]", rosterReviewOfferingID), row.Sources)
		assert.Zero(t, row.Targets, "the Klasse target no longer adds children")
		for _, instanceID := range fixture.occurrences[segment] {
			assert.Equal(t, fixture.booked, s.occurrenceStudents(t, instanceID),
				"segment %d occurrence %d keeps only the booked children", segment, instanceID)
		}
	}

	// History stays: segment A's hand rows end at the boundary.
	var openHandRows int
	require.NoError(t, s.db.NewRaw(`SELECT COUNT(*) FROM activities.student_enrollments
		WHERE activity_group_id IN (?, ?) AND enrollment_request_child_id IS NULL
			AND selected_weekdays IS NULL AND (valid_until IS NULL OR valid_until > GREATEST(valid_from, ?::date))`,
		fixture.segmentA, fixture.segmentB, effective.String()).Scan(s.ctx, &openHandRows))
	assert.Zero(t, openHandRows, "no hand-picked child plans the series after the boundary")
}

func TestTemplateOfferingConnectionRefusals(t *testing.T) {
	t.Parallel()
	s := buildRosterReviewModule(t, &rosterMaintenanceSources{})
	fixture := s.arrangeTwoSegmentSeries(t)
	announcer := &recordingStaffingAnnouncer{}
	s.res.Staffing = announcer
	path := fmt.Sprintf("/templates/%d/offering-connection", fixture.segmentB)
	for name, body := range map[string]map[string]any{
		"past date":           {"offering_ids": []int64{rosterReviewOfferingID}, "effective_date": calendar.NewDate(2020, 1, 6).String()},
		"no offering":         {"offering_ids": []int64{}, "effective_date": rosterReviewMonday.String()},
		"grade and class":     {"offering_ids": []int64{rosterReviewOfferingID}, "grade_levels": []int{2}, "school_classes": []string{"2c"}, "effective_date": rosterReviewMonday.String()},
		"malformed date":      {"offering_ids": []int64{rosterReviewOfferingID}, "effective_date": "04.03.2030"},
		"preview in the past": nil,
	} {
		t.Run(name, func(t *testing.T) {
			if body == nil {
				w := doTemplateJSON(t, s.router, http.MethodGet, fmt.Sprintf("/templates/%d/offering-preview?offering_ids=%d&from=%s",
					fixture.segmentB, rosterReviewOfferingID, rosterReviewMonday.AddDays(-1).String()), nil)
				assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
				return
			}
			w := doTemplateJSON(t, s.router, http.MethodPost, path, body)
			assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
		})
	}
	assert.Empty(t, s.resyncs, "a refused connection writes nothing")
	assert.Empty(t, announcer.sources, "a refused connection announces nothing")
}

// recordingStaffingAnnouncer records the staffing announcements a handler
// queues; the test router's transaction runs them after its commit.
type recordingStaffingAnnouncer struct {
	mu      sync.Mutex
	sources []string
}

func (a *recordingStaffingAnnouncer) AnnounceStaffingChanged(_ int64, source string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sources = append(a.sources, source)
	return nil
}

// TestTemplateRosterDeviationsQueryBudget keeps the deviation list batched:
// the templates, their indicator, the bookings and the future rosters are
// read once for the period, not per Regeltermin. The bookings come from a
// stub; Care Plan's read has its own budget in the enrollment suite.
func TestTemplateRosterDeviationsQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	sources := &rosterMaintenanceSources{}
	s := buildRosterReviewModule(t, sources)
	period := createTemplateTestPeriodRange(t, s.db, "Review-Budget", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	class := createClassStudents(t, s.db, "4b", 5)
	sources.bookings = rosterReviewBookings(rosterReviewOfferingID, "Randstunde", "4b", class[:4], 1, 3)
	counter := testpkg.CaptureQueries(t, s.db)
	measured := chi.NewRouter()
	measured.Use(render.SetContentType(render.ContentTypeJSON))
	measured.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(testpkg.WithTestTenantRuntime(t, r.Context())))
		})
	})
	measured.Get("/templates/roster-deviations", s.res.listTemplateRosterDeviations)
	router := testpkg.MeasuredTenantRequest(t, s.db, testpkg.TenantIDFromContext(s.ctx), counter, measured)

	created := 0
	addTemplates := func(n int) {
		for range n {
			s.createClassTemplate(t, fmt.Sprintf("Budget-RDST-%d", created), "4b", period.ID, class)
			created++
		}
	}
	// granted empty: a caller without the right to read children, who gets
	// the counts only; "*:*": an administrator, who also gets the names.
	run := func(granted ...string) int {
		counter.Reset()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/templates/roster-deviations?period_id=%d", period.ID), nil)
		req = req.WithContext(testpkg.WithPermissionsContext(req.Context(), granted))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		deviations := decodeTemplateData[rosterDeviationsResponse](t, w).Deviations
		require.Len(t, deviations, created)
		named := 0
		if len(granted) > 0 {
			named = 1 // class[4] books nothing
		}
		for _, deviation := range deviations {
			assert.Len(t, deviation.Unbooked, named)
		}
		return counter.Total()
	}

	addTemplates(3)
	small, smallNamed := run(), run("*:*")
	addTemplates(5)
	large := run()
	unnamed := counter.Queries()
	largeNamed := run("*:*")

	t.Logf("query budget: 3 templates → %d (named %d) queries, 8 templates → %d (named %d) queries", small, smallNamed, large, largeNamed)
	assert.Equal(t, small, large, "query count must not grow with the number of Regeltermine")
	assert.Equal(t, smallNamed, largeNamed, "the names are read in one batch for every Regeltermin")
	querybudgets.AssertQueryBudget(t, "api.timetable.templates.roster_deviations", unnamed)
	querybudgets.AssertQueryBudget(t, "api.timetable.templates.roster_deviations.named", counter.Queries())
}

// TestTemplateRosterDeviationsTiedCombinations (review of #5363): two
// combinations explain the same children of a hand-kept Klasse
// Regeltermin. The hint carries both and preselects neither.
func TestTemplateRosterDeviationsTiedCombinations(t *testing.T) {
	t.Parallel()
	sources := &rosterMaintenanceSources{results: map[int64]timetableModule.TemplateRosterMaintenance{}}
	s := buildRosterReviewModule(t, sources)
	period := createTemplateTestPeriodRange(t, s.db, "Review-Period", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	class := createClassStudents(t, s.db, "1c", 7)
	first := rosterReviewOfferingID
	offerings := []templateRosterMaintenanceOfferingResponse{
		{ID: first, Name: "A"}, {ID: first + 1, Name: "B"}, {ID: first + 2, Name: "C"}, {ID: first + 3, Name: "D"},
	}
	pick := func(indexes ...int) []int64 {
		ids := make([]int64, 0, len(indexes))
		for _, index := range indexes {
			ids = append(ids, class[index])
		}
		return ids
	}
	sources.bookings = append(rosterReviewBookings(first, "A", "1c", pick(0, 1, 2), 1, 3),
		rosterReviewBookings(first+1, "B", "1c", pick(3, 4, 5), 1, 3)...)
	sources.bookings = append(sources.bookings, rosterReviewBookings(first+2, "C", "1c", pick(0, 3, 4), 1, 3)...)
	sources.bookings = append(sources.bookings, rosterReviewBookings(first+3, "D", "1c", pick(1, 2, 5), 1, 3)...)
	template := s.createClassTemplate(t, "RDST 1c", "1c", period.ID, class)
	sources.results[template] = timetableModule.TemplateRosterMaintenance{Mode: "manual", DynamicTargetsManual: true}

	deviations := s.listDeviations(t, period.ID)

	require.Len(t, deviations, 1)
	got := deviations[0]
	assert.True(t, got.Ambiguous)
	assert.ElementsMatch(t, [][]templateRosterMaintenanceOfferingResponse{
		{offerings[0], offerings[1]}, {offerings[2], offerings[3]},
	}, got.Combinations)
	assert.ElementsMatch(t, offerings, got.Candidates)
}
