package timetablehttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// fieldErrorBody is one entry of the errors list.
type fieldErrorBody struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// renderedError is the wire body of one error answer.
type renderedError struct {
	Status  int              `json:"-"`
	Code    string           `json:"code"`
	Errors  []fieldErrorBody `json:"errors"`
	Details map[string]any   `json:"details"`
	Error   string           `json:"error"`
}

func renderError(t *testing.T, renderer render.Renderer) renderedError {
	t.Helper()
	require.NotNil(t, renderer)
	w := httptest.NewRecorder()
	if recorded, ok := renderer.(recordedRenderer); ok {
		w = recorded.w
	} else {
		require.NoError(t, render.Render(w, httptest.NewRequest(http.MethodPost, "/", nil), renderer))
	}
	var body renderedError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	body.Status = w.Code
	return body
}

func fieldNames(errs []fieldErrorBody) []string {
	names := make([]string, 0, len(errs))
	for _, e := range errs {
		names = append(names, e.Field)
	}
	return names
}

type codeCase struct {
	name       string
	renderer   render.Renderer
	wantStatus int
	wantCode   string
	wantField  string
	wantDetail map[string]any
}

func assertCodeCases(t *testing.T, cases []codeCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := renderError(t, tc.renderer)
			assert.Equal(t, tc.wantStatus, body.Status)
			assert.Equal(t, tc.wantCode, body.Code)
			if tc.wantField == "" {
				assert.Empty(t, body.Errors)
			} else {
				assert.Equal(t, []string{tc.wantField}, fieldNames(body.Errors))
			}
			if tc.wantDetail == nil {
				assert.Empty(t, body.Details)
			} else {
				assert.Equal(t, tc.wantDetail, body.Details)
			}
		})
	}
}

// TestTimetableCodeConstantsAreRegistered pins every wire code the owner and
// its compositions raise below the adapter to error-registry.json (#2516): a
// typo there would reach the client as an unknown code.
func TestTimetableCodeConstantsAreRegistered(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../../../error-registry.json")
	require.NoError(t, err)
	var registry struct {
		Codes []struct {
			Code string `json:"code"`
		} `json:"codes"`
	}
	require.NoError(t, json.Unmarshal(raw, &registry))
	registered := make(map[string]bool, len(registry.Codes))
	for _, entry := range registry.Codes {
		registered[entry.Code] = true
	}
	for _, code := range []string{
		timetable.CodeOfferingSourceInvalid,
		timetable.CodeOfferingSourceTooMany,
		timetable.CodeOfferingSourceNotFound,
		timetable.CodeOfferingSourceInactive,
		timetable.CodeOfferingSourceMixedPhases,
		timetable.CodeOfferingSourceOutsidePeriod,
		timetable.CodeTemplateGradeAboveMax,
		timetable.CodeSeriesEndBeforeStart,
		timetable.CodeSeriesEndOutsidePeriod,
		timetable.CodeTemplateSplitInPast,
		timetable.CodeOperationStale,
		timetable.CodeInstanceNotActive,
		timetable.CodeAttendanceFrozen,
		timetable.CodeReopenStudentActive,
		timetable.CodeReopenAttendanceChanged,
		timetable.CodeReopenSupervisionChanged,
		timetable.CodeReopenStaffBusy,
		timetable.CodeGuardianNoticeTextInvalid,
		timetable.CodeGuardianNoticePast,
		timetable.CodeStudentNotCheckedIn,
		timetable.CodeWindowEndBeforeStart,
		timetable.CodeWindowTooLarge,
		timetable.CodeInstanceWeekend,
		timetable.CodeInstanceInPast,
		timetable.CodeDeviationNoteTooLong,
		timetable.CodeDeviationSelectionInvalid,
		timetable.CodeStaffNotFound,
		timetable.CodeSubstituteSingleOnly,
		timetable.CodeSubstituteSelf,
		timetable.CodeSubstituteAbsent,
		timetable.CodeSubstituteAbsentOnDate,
		timetable.CodeStaffPresentAndAbsent,
		timetable.CodeSubstituteNotOnInstances,
		timetable.CodeInstancesRequired,
		timetable.CodeInstanceSelectionInvalid,
		timetable.CodeInstancesOtherDay,
		timetable.CodeInstancesNotAssigned,
		timetable.CodeMoveSameInstance,
		timetable.CodeMoveOtherDay,
		timetable.CodeStaffNotOnSource,
		timetable.CodeStaffAbsentOnSource,
		timetable.CodeDatesRequired,
		timetable.CodeDatesInPast,
		timetable.CodeTooManyDates,
		timetable.CodeDeviationInstanceNotFound,
	} {
		assert.True(t, registered[code], "%s is not in error-registry.json", code)
	}
}

// TestCalendarPeriodErrorCodes pins the codes, statuses and fields of the
// calendar period editor (#2516); an invalid period used to answer 500.
func TestCalendarPeriodErrorCodes(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	bind := func(req CalendarPeriodRequest) render.Renderer {
		return bindErrorRenderer(req.Bind(r))
	}
	invalid := &schoolcalendar.InvalidCalendarPeriodError{Reason: "end_date must be after start_date"}
	assertCodeCases(t, []codeCase{
		{"missing name", bind(CalendarPeriodRequest{PeriodType: "custom", StartDate: "2026-01-01", EndDate: "2026-02-01"}),
			http.StatusBadRequest, "timetable.calendar_period_invalid", "name", nil},
		{"unknown type", bind(CalendarPeriodRequest{Name: "Herbst", PeriodType: "nope", StartDate: "2026-01-01", EndDate: "2026-02-01"}),
			http.StatusBadRequest, "timetable.calendar_period_invalid", "period_type", nil},
		{"missing end", bind(CalendarPeriodRequest{Name: "Herbst", PeriodType: "custom", StartDate: "2026-01-01"}),
			http.StatusBadRequest, "timetable.calendar_period_invalid", "end_date", nil},
		{"name taken", calendarPeriodWriteErrorRenderer(r, fmt.Errorf("add: %w", schoolcalendar.ErrCalendarPeriodNameConflict), "x"),
			http.StatusConflict, "timetable.calendar_period_name_taken", "name", nil},
		{"invalid period no longer 500", calendarPeriodWriteErrorRenderer(r, invalid, "x"),
			http.StatusBadRequest, "timetable.calendar_period_invalid", "", nil},
		{"vanished period", calendarPeriodWriteErrorRenderer(r, schoolcalendar.ErrCalendarPeriodNotFound, "x"),
			http.StatusNotFound, "timetable.calendar_period_not_found", "", nil},
		{"other failure", calendarPeriodWriteErrorRenderer(r, errors.New("db down"), "x"),
			http.StatusInternalServerError, "general.server", "", nil},
		{"end before start", invalidOnField("timetable.calendar_period_end_before_start", "end_date", "end_date must be after start_date"),
			http.StatusBadRequest, "timetable.calendar_period_end_before_start", "end_date", nil},
		{"broken body stays general", bindErrorRenderer(errors.New("EOF")),
			http.StatusBadRequest, "general.input", "", nil},
	})
}

// TestClosingDayErrorCodes pins the closing-day refusals; every service
// failure used to answer 500.
func TestClosingDayErrorCodes(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	invalid := &schoolcalendar.InvalidClosingDayError{Reason: "reason is required"}
	assertCodeCases(t, []codeCase{
		{"missing reason", bindErrorRenderer((&ClosingDayRequest{StartDate: "2026-01-01", EndDate: "2026-01-02"}).Bind(r)),
			http.StatusBadRequest, "timetable.closing_day_invalid", "reason", nil},
		{"invalid from service", closingDayWriteErrorRenderer(invalid, "x"),
			http.StatusBadRequest, "timetable.closing_day_invalid", "", nil},
		{"vanished", closingDayWriteErrorRenderer(fmt.Errorf("update: %w", schoolcalendar.ErrClosingDayNotFound), "x"),
			http.StatusNotFound, "timetable.closing_day_not_found", "", nil},
		{"other failure", closingDayWriteErrorRenderer(errors.New("db down"), "x"),
			http.StatusInternalServerError, "general.server", "", nil},
	})
}

// TestPlanningTrackErrorCodes pins the planning-track editor refusals.
func TestPlanningTrackErrorCodes(t *testing.T) {
	t.Parallel()

	_, invalidColor := timetable.PlanningTrackDraft{Name: "Jahrgang 1", Color: "blue"}.Validate()
	rules := func(err error) render.Renderer {
		w := httptest.NewRecorder()
		renderPlanningTrackError(w, httptest.NewRequest(http.MethodPost, "/", nil), err)
		return recordedRenderer{w}
	}
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	assertCodeCases(t, []codeCase{
		{"bind color", bindErrorRenderer((&planningTrackRequest{Name: "A"}).Bind(r)),
			http.StatusBadRequest, "timetable.planning_track_invalid", "color", nil},
		{"invalid color", rules(invalidColor), http.StatusBadRequest, "timetable.planning_track_invalid", "color", nil},
		{"archived", rules(timetable.ErrPlanningTrackArchived), http.StatusBadRequest, "timetable.planning_track_archived", "", nil},
		{"name taken", rules(timetable.ErrPlanningTrackNameTaken), http.StatusConflict, "timetable.planning_track_name_taken", "name", nil},
		{"not found", rules(timetable.ErrPlanningTrackNotFound), http.StatusNotFound, "timetable.planning_track_not_found", "", nil},
	})
	require.ErrorIs(t, invalidColor, timetable.ErrInvalidPlanningTrack)
}

// TestTemplateRefusalCodes pins the refusals every Regeltermin write shares,
// including the values the offering-source and grade refusals name.
func TestTemplateRefusalCodes(t *testing.T) {
	t.Parallel()

	tooMany := timetable.WithCode(fmt.Errorf("%w: at most 5", timetable.ErrOfferingSourceInvalid),
		timetable.CodeOfferingSourceTooMany, timetable.RefusalValues{Max: 5, Given: 6})
	grade := timetable.WithCode(fmt.Errorf("%w: target_grade_level 7 exceeds tenant maximum 4", timetable.ErrTemplateTargetGradeExceedsLimit),
		timetable.CodeTemplateGradeAboveMax, timetable.RefusalValues{Grade: 7, Max: 4})
	assertCodeCases(t, []codeCase{
		{"category", templateRefusalRenderer(fmt.Errorf("op: %w", timetable.ErrCategoryNotAssignable)),
			http.StatusBadRequest, "timetable.template_category_unavailable", "category_id", nil},
		{"planning track", templateRefusalRenderer(timetable.ErrPlanningTrackArchived),
			http.StatusBadRequest, "timetable.template_planning_track_unavailable", "planning_track_id", nil},
		{"weekend", templateRefusalRenderer(timetable.ErrTemplateWeekendWeekday),
			http.StatusBadRequest, "timetable.template_weekend", "weekdays", nil},
		{"too many sources", templateRefusalRenderer(tooMany),
			http.StatusBadRequest, "timetable.offering_source_too_many", "", map[string]any{"max": float64(5), "given": float64(6)}},
		{"uncoded source", templateRefusalRenderer(fmt.Errorf("%w: student_ids must be empty", timetable.ErrOfferingSourceInvalid)),
			http.StatusBadRequest, "timetable.offering_source_invalid", "", nil},
		{"education group", templateRefusalRenderer(&timetable.TemplateEducationGroupError{Err: errors.New("no group")}),
			http.StatusBadRequest, "timetable.template_education_group_invalid", "education_group_id", nil},
		{"grade above max", templateRefusalRenderer(grade),
			http.StatusBadRequest, "timetable.template_grade_above_max", "", map[string]any{"grade": float64(7), "max": float64(4)}},
		{"split in past", templateSplitInvalidRenderer(timetable.WithCode(timetable.ErrSplitInvalidInput, timetable.CodeTemplateSplitInPast)),
			http.StatusBadRequest, "timetable.template_split_in_past", "", nil},
		{"split invalid", templateSplitInvalidRenderer(fmt.Errorf("%w: bounded", timetable.ErrSplitInvalidInput)),
			http.StatusBadRequest, "timetable.template_split_invalid", "", nil},
	})
	assert.Nil(t, templateRefusalRenderer(errors.New("db down")))
}

// TestSeriesEndAndStartCodes pins the date refusals of a template series.
func TestSeriesEndAndStartCodes(t *testing.T) {
	t.Parallel()

	periodEnd := calendar.Date("2026-07-15")
	beforeStart := timetable.ValidateSeriesLastDay("2026-03-01", "2026-03-10", nil)
	afterPeriod := timetable.ValidateSeriesLastDay("2026-08-01", "2026-03-10", &periodEnd)
	outside := timetable.WithCode(fmt.Errorf("%w (2026-08-01 to 2027-07-31)", errTemplateStartDateOutsidePeriod),
		"timetable.template_start_outside_period", timetable.RefusalValues{Start: "01.08.2026", End: "31.07.2027"})
	assertCodeCases(t, []codeCase{
		{"end before start", codedInvalidOnField(beforeStart, "end_date"),
			http.StatusBadRequest, "timetable.series_end_before_start", "end_date", map[string]any{"start": "10.03.2026"}},
		{"end after period", codedInvalidOnField(afterPeriod, "end_date"),
			http.StatusBadRequest, "timetable.series_end_outside_period", "end_date", map[string]any{"end": "15.07.2026"}},
		{"start outside period", codedInvalidOnField(outside, "start_date"),
			http.StatusBadRequest, "timetable.template_start_outside_period", "start_date",
			map[string]any{"start": "01.08.2026", "end": "31.07.2027"}},
	})
	require.ErrorIs(t, beforeStart, timetable.ErrInvalidSeriesEnd)
}

// TestOperationErrorCodes pins the operational refusals of both the planner
// and the operations path, which used to answer without codes (#2516).
func TestOperationErrorCodes(t *testing.T) {
	t.Parallel()

	staffBusy := timetable.WithCode(fmt.Errorf("%w: staff now supervises another group", timetable.ErrTimetableOperationConflict),
		timetable.CodeReopenStaffBusy)
	notCheckedIn := timetable.WithCode(timetable.ErrTimetableOperationNotFound, timetable.CodeStudentNotCheckedIn)
	assertCodeCases(t, []codeCase{
		{"start too early", operationErrorRenderer(timetable.ErrInstanceStartTooEarly), http.StatusConflict, "timetable.start_too_early", "", nil},
		{"start expired", operationErrorRenderer(timetable.ErrInstanceStartExpired), http.StatusConflict, "timetable.start_window_expired", "", nil},
		{"complete early", operationErrorRenderer(timetable.ErrInstanceCompleteEarly), http.StatusConflict, "timetable.complete_too_early", "", nil},
		{"transition", operationErrorRenderer(timetable.ErrInvalidInstanceTransition), http.StatusConflict, "timetable.invalid_transition", "", nil},
		{"reopen staff busy", operationErrorRenderer(staffBusy), http.StatusConflict, "timetable.reopen_staff_busy", "", nil},
		{"uncoded conflict", operationErrorRenderer(timetable.ErrTimetableOperationConflict), http.StatusConflict, "timetable.operation_stale", "", nil},
		{"no staff profile", operationErrorRenderer(timetable.ErrNoStaffProfile), http.StatusForbidden, "timetable.no_staff_profile", "", nil},
		{"not planned", operationErrorRenderer(timetable.ErrTimetableOperationForbidden), http.StatusForbidden, "timetable.operation_not_planned", "", nil},
		{"not checked in", operationErrorRenderer(notCheckedIn), http.StatusNotFound, "timetable.student_not_checked_in", "", nil},
		{"operation not found", operationErrorRenderer(timetable.ErrTimetableOperationNotFound), http.StatusNotFound, "timetable.instance_not_found", "", nil},
		{"weekend", operationErrorRenderer(timetable.ErrInstanceWeekend), http.StatusBadRequest, "timetable.instance_weekend", "", nil},
		{"child active", operationErrorRenderer(studentpresence.ErrStudentAlreadyActive), http.StatusConflict, "timetable.student_already_active", "", nil},
		{"room occupied", operationErrorRenderer(studentpresence.ErrRoomConflict), http.StatusConflict, "timetable.room_occupied", "", nil},
		{"none present", operationErrorRenderer(studentpresence.ErrStudentsNotPresent), http.StatusConflict, "timetable.students_not_present", "", nil},
		{"group ended", operationErrorRenderer(studentpresence.ErrGroupAlreadyEnded), http.StatusConflict, "timetable.group_already_ended", "", nil},
		{"graduated", operationErrorRenderer(studentpresence.ErrStudentGraduated), http.StatusNotFound, "timetable.student_graduated", "", nil},
		{"care ended", operationErrorRenderer(studentpresence.ErrStudentCareEnded), http.StatusNotFound, "timetable.student_care_ended", "", nil},
		{"unknown stays 500", operationErrorRenderer(errors.New("db down")), http.StatusInternalServerError, "general.server", "", nil},
	})
}

// TestInstanceLifecycleErrorCodes pins the planner-side codes, including the
// cancellation notice refusals that used to be bare 400s.
func TestInstanceLifecycleErrorCodes(t *testing.T) {
	t.Parallel()

	renderErr := func(err error) render.Renderer {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		renderInstanceLifecycleError(w, r, err)
		return recordedRenderer{w}
	}
	past := timetable.WithCode(fmt.Errorf("%w: block is in the past", timetable.ErrGuardianNoticeInvalid), timetable.CodeGuardianNoticePast)
	assertRecorded(t, []recordedCase{
		{"notice past", renderErr(past), http.StatusBadRequest, "timetable.guardian_notice_past"},
		{"notice publish refused", renderErr(fmt.Errorf("%w: no students", timetable.ErrGuardianNoticeInvalid)), http.StatusBadRequest, "timetable.guardian_notice_invalid"},
		{"outside period", renderErr(timetable.ErrInstanceOutsideActiveCalendarPeriod), http.StatusBadRequest, "timetable.instance_outside_period"},
		{"not found", renderErr(timetable.ErrInstanceNotFound), http.StatusNotFound, "timetable.instance_not_found"},
		{"moved keeps its code", renderErr(timetable.ErrInstanceMoved), http.StatusConflict, "timetable.instance_moved"},
		{"unknown stays 500", renderErr(errors.New("db down")), http.StatusInternalServerError, "general.server"},
	})
}

// recordedRenderer carries an already written answer through the shared
// assertion.
type recordedRenderer struct{ w *httptest.ResponseRecorder }

func (recordedRenderer) Render(http.ResponseWriter, *http.Request) error { return nil }

type recordedCase struct {
	name       string
	recorded   render.Renderer
	wantStatus int
	wantCode   string
}

func assertRecorded(t *testing.T, cases []recordedCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := tc.recorded.(recordedRenderer).w
			var body renderedError
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tc.wantStatus, w.Code)
			assert.Equal(t, tc.wantCode, body.Code)
		})
	}
}

// TestDeviationRefusalCodes pins that a Vertretungsplan refusal keeps its
// status and carries its code, the values it names and its field.
func TestDeviationRefusalCodes(t *testing.T) {
	t.Parallel()

	renderErr := func(err error) render.Renderer {
		w := httptest.NewRecorder()
		renderDeviationError(w, httptest.NewRequest(http.MethodPost, "/", nil), err)
		return recordedRenderer{w}
	}
	absentOnDate := timetable.DeviationBadRequest("die Ersatzperson ist am 06.10.2026 selbst abwesend").
		WithCode(timetable.CodeSubstituteAbsentOnDate).OnField("substitute_staff_id").WithValues(timetable.RefusalValues{Date: "2026-10-06"})
	w := renderErr(absentOnDate).(recordedRenderer).w
	var body renderedError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "timetable.substitute_absent_on_date", body.Code)
	assert.Equal(t, map[string]any{"date": "2026-10-06"}, body.Details)
	assert.Equal(t, []string{"substitute_staff_id"}, fieldNames(body.Errors))
	assert.Equal(t, "die Ersatzperson ist am 06.10.2026 selbst abwesend", body.Error)

	assertRecorded(t, []recordedCase{
		{"not found", renderErr(timetable.DeviationNotFound("der Termin wurde nicht gefunden").WithCode(timetable.CodeDeviationInstanceNotFound)),
			http.StatusNotFound, "timetable.instance_not_found"},
		{"self substitution", renderErr(timetable.DeviationBadRequest("x").WithCode(timetable.CodeSubstituteSelf)),
			http.StatusBadRequest, "timetable.substitute_self"},
		{"conflict keeps its code", renderErr(timetable.DeviationConflict(timetable.CodeSubstituteConflict, "x")),
			http.StatusConflict, "timetable.substitute_conflict"},
		{"uncoded 400 keeps the class", renderErr(timetable.DeviationBadRequest("x")),
			http.StatusBadRequest, "general.input"},
	})
}

// TestWindowAndRangeCodes pins the range refusals of materialize/re-plan,
// bulk cancel, the plan export and the gap reads with their limits.
func TestWindowAndRangeCodes(t *testing.T) {
	t.Parallel()

	from, to := "2026-01-05", "2026-04-30"
	_, _, tooLarge := resolveMaterializationWindow(&materializeRequest{FromDate: &from, ToDate: &to}, time.Now())
	_, _, reversed := resolveMaterializationWindow(&materializeRequest{FromDate: &to, ToDate: &from}, time.Now())
	bulkTooLong := timetable.ValidateBulkCancelRange("2026-01-01", "2027-06-01")
	weekend := validateTimetableWorkday(context.Background(), calendar.Date("2026-10-10"))
	assertCodeCases(t, []codeCase{
		{"window too large", codedInvalid(tooLarge), http.StatusBadRequest, "timetable.window_too_large", "",
			map[string]any{"max_days": float64(timetable.MaxMaterializationWindowDays)}},
		{"window reversed", codedInvalid(reversed), http.StatusBadRequest, "timetable.window_end_before_start", "", nil},
		{"bulk cancel too long", codedInvalid(bulkTooLong), http.StatusBadRequest, "timetable.window_too_large", "",
			map[string]any{"max_days": float64(timetable.MaxBulkCancelDays)}},
		{"weekend date", codedInvalidOnField(weekend, "date"), http.StatusBadRequest, "timetable.instance_weekend", "date", nil},
		{"export too large", planExportInvalidRenderer(planexport.ErrRangeTooLarge), http.StatusBadRequest, "timetable.export_range_too_large", "",
			map[string]any{"max_weeks": float64(planexport.MaxExportWeeks)}},
		{"export reversed", planExportInvalidRenderer(planexport.ErrRangeReversed), http.StatusBadRequest, "timetable.export_end_before_start", "to", nil},
		{"export other", planExportInvalidRenderer(fmt.Errorf("%w: unknown variant", planexport.ErrInvalidParams)), http.StatusBadRequest, "general.input", "", nil},
	})
	require.ErrorIs(t, bulkTooLong, timetable.ErrInvalidBulkCancelRange)
}

// TestSlotListRefusalCodes pins the codes of the daily lists (#2516).
func TestSlotListRefusalCodes(t *testing.T) {
	t.Parallel()

	assertCodeCases(t, []codeCase{
		{"pickup list in the past", slotListRefusal(classday.ErrPickupCohortPastDate), http.StatusBadRequest, "timetable.slot_list_pickup_past_date", "", nil},
		{"reconciliation in the future", slotListRefusal(classday.ErrReconciliationFutureDate), http.StatusBadRequest, "timetable.slot_list_reconciliation_future_date", "", nil},
		{"drifted", slotListRefusal(fmt.Errorf("render: %w", classday.ErrListDrifted)), http.StatusConflict, "timetable.slot_list_drifted", "", nil},
		{"disabled", slotListRefusal(classday.ErrTimetableDisabled), http.StatusForbidden, "timetable.slot_lists_disabled", "", nil},
	})
	assert.Nil(t, slotListRefusal(errors.New("db down")))
}

// TestAttendanceCorrectionCodes pins the correction refusals.
func TestAttendanceCorrectionCodes(t *testing.T) {
	t.Parallel()

	rs := &Resource{}
	renderErr := func(err error) render.Renderer {
		w := httptest.NewRecorder()
		rs.renderCorrectionError(w, httptest.NewRequest(http.MethodPost, "/", nil), err)
		return recordedRenderer{w}
	}
	assertRecorded(t, []recordedCase{
		{"reason missing", renderErr(timetable.ErrCorrectionReasonRequired), http.StatusBadRequest, "timetable.correction_reason_missing"},
		{"reason too long", renderErr(timetable.ErrCorrectionReasonTooLong), http.StatusBadRequest, "timetable.correction_reason_too_long"},
		{"not completed", renderErr(timetable.ErrCorrectionRequiresCompleted), http.StatusConflict, "timetable.correction_not_completed"},
		{"cancelled", renderErr(timetable.ErrCorrectionCancelled), http.StatusConflict, "timetable.correction_cancelled"},
		{"entry missing", renderErr(timetable.ErrAttendanceEntryNotFound), http.StatusNotFound, "timetable.attendance_entry_not_found"},
	})
}
