package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type failAfterOfferingWriteCoordinator struct {
	enrollmentService.DirectOfferingAdjustmentCoordinator
}

func (c failAfterOfferingWriteCoordinator) ApplyDirectOfferingAdjustment(
	ctx context.Context,
	input enrollmentService.DirectOfferingAdjustmentInput,
) error {
	if err := c.DirectOfferingAdjustmentCoordinator.ApplyDirectOfferingAdjustment(ctx, input); err != nil {
		return err
	}
	return enrollmentService.ErrPickupAdjustmentInvalid
}

func TestPickupAdjustmentProtectedRouterRequiresExplicitExceptionAndAuditsApply(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	student := testpkg.CreateTestStudent(t, tc.db, "PickupAdjustment", "Child", "PA1")
	staff, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "PickupAdjustment", "Staff")
	require.NoError(t, tc.services.Settings.SetValue(
		testpkg.Ctx(t), configModels.KeyRequirePickupOfferingReview, true, nil, nil,
	))
	effectiveFrom := timezone.TodayDate().String()
	body := map[string]any{
		"schedules":      []map[string]any{{"weekday": 1, "pickup_time": "13:45"}},
		"care_days":      []int{1},
		"effective_from": effectiveFrom,
	}

	readOnlyReq := testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/%d/pickup-schedules/preview", student.ID), body,
	)
	readOnlyRec := authExec(
		t, tc, readOnlyReq, testutil.TeacherTestClaims(int(account.ID)), []string{"users:read"},
	)
	assert.Equal(t, http.StatusForbidden, readOnlyRec.Code, readOnlyRec.Body.String())
	mismatchedArrivalBody := cloneMap(body)
	mismatchedArrivalBody["arrival_schedules"] = []map[string]any{{
		"weekday": scheduleModels.WeekdayTuesday, "expected_arrival": "08:00",
	}}
	mismatchedArrivalReq := testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/%d/pickup-schedules/preview", student.ID), mismatchedArrivalBody,
	)
	mismatchedArrivalRec := authExec(
		t, tc, mismatchedArrivalReq, testutil.AdminTestClaims(int(account.ID)), []string{"users:update"},
	)
	assert.Equal(t, http.StatusBadRequest, mismatchedArrivalRec.Code, mismatchedArrivalRec.Body.String())
	assert.Contains(t, mismatchedArrivalRec.Body.String(), `"code":"pickup.invalid"`)

	previewReq := testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/%d/pickup-schedules/preview", student.ID), body,
	)
	previewRec := authExec(
		t, tc, previewReq, testutil.AdminTestClaims(int(account.ID)), []string{"users:update"},
	)
	require.Equal(t, http.StatusOK, previewRec.Code, previewRec.Body.String())
	var previewEnvelope struct {
		Data struct {
			PreviewToken       string `json:"preview_token"`
			ResolutionRequired bool   `json:"resolution_required"`
			MatchingOfferings  []any  `json:"matching_offerings"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(previewRec.Body.Bytes(), &previewEnvelope))
	assert.True(t, previewEnvelope.Data.ResolutionRequired)
	assert.Empty(t, previewEnvelope.Data.MatchingOfferings)
	require.NotEmpty(t, previewEnvelope.Data.PreviewToken)

	withoutDecision := cloneMap(body)
	withoutDecision["preview_token"] = previewEnvelope.Data.PreviewToken
	applyReq := testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/%d/pickup-schedules/apply", student.ID), withoutDecision,
	)
	applyRec := authExec(
		t, tc, applyReq, testutil.AdminTestClaims(int(account.ID)), []string{"users:update"},
	)
	assert.Equal(t, http.StatusBadRequest, applyRec.Code, applyRec.Body.String())
	assert.Contains(t, applyRec.Body.String(), `"code":"pickup.resolution_required"`)

	manualTime, err := time.Parse("2006-01-02 15:04", "2000-01-01 14:00")
	require.NoError(t, err)
	manual := &scheduleModels.StudentPickupSchedule{
		StudentID: student.ID, Weekday: 1, PickupTime: manualTime, CreatedBy: staff.StaffID,
		Source: scheduleModels.PickupScheduleSourceStaff,
	}
	manual.SetTenantID(student.TenantID)
	repoFactory := repositories.NewFactory(tc.db)
	require.NoError(t, repoFactory.StudentPickupSchedule.UpsertSchedule(testpkg.Ctx(t), manual))

	withDecision := cloneMap(withoutDecision)
	withDecision["resolution"] = "exception"
	staleReq := testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/%d/pickup-schedules/apply", student.ID), withDecision,
	)
	staleRec := authExec(
		t, tc, staleReq, testutil.AdminTestClaims(int(account.ID)), []string{"users:update"},
	)
	assert.Equal(t, http.StatusConflict, staleRec.Code, staleRec.Body.String())
	assert.Contains(t, staleRec.Body.String(), `"code":"pickup.preview_stale"`)
	stored, err := repoFactory.StudentPickupSchedule.FindByStudentIDAndWeekday(
		testpkg.Ctx(t), student.ID, 1,
	)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, "14:00", stored.PickupTime.Format("15:04"))
	history, err := tc.services.StudentAudit.GetChangeHistory(testpkg.Ctx(t), student.ID)
	require.NoError(t, err)
	assert.Empty(t, history, "a stale preview must not write an audit row")

	freshPreview := postPickupAdjustmentPreview(t, tc, student.ID, account.ID, body)
	withDecision["preview_token"] = freshPreview.PreviewToken
	confirmedReq := testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/%d/pickup-schedules/apply", student.ID), withDecision,
	)
	confirmedRec := authExec(
		t, tc, confirmedReq, testutil.AdminTestClaims(int(account.ID)), []string{"users:update"},
	)
	require.Equal(t, http.StatusOK, confirmedRec.Code, confirmedRec.Body.String())

	history, err = tc.services.StudentAudit.GetChangeHistory(testpkg.Ctx(t), student.ID)
	require.NoError(t, err)
	require.NotEmpty(t, history)
	assert.Equal(t, auditModels.StudentFieldPickupSchedule, history[0].FieldName)
	assert.Contains(t, valueOrEmpty(history[0].NewValue), "Dauerhafte Ausnahme")
}

func TestPickupAdjustmentProtectedRouterChangesMatchingOfferingThroughSharedPath(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	student := testpkg.CreateTestStudent(t, tc.db, "PickupOffer", "Child", "PO1")
	staff, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "PickupOffer", "Staff")
	fixture := setupCorrectionFixture(t, tc, student.ID, student.TenantID, "Child")
	require.NoError(t, tc.services.Settings.SetValue(
		testpkg.Ctx(t), configModels.KeyRequirePickupOfferingReview, true, nil, nil,
	))
	for offering, pickupTime := range map[*enrollmentModels.CareOffering]string{
		fixture.ganztag: "16:00",
		fixture.mittag:  "14:30",
	} {
		pickupTimes := map[string]string{
			"mon": pickupTime, "tue": pickupTime, "wed": pickupTime, "thu": pickupTime, "fri": pickupTime,
		}
		_, err := tc.db.NewUpdate().Model(offering).
			ModelTableExpr(`enrollment.care_offerings AS "care_offering"`).
			Set("pickup_times = ?", pickupTimes).
			WherePK().Exec(t.Context())
		require.NoError(t, err)
	}
	_, err := tc.db.NewDelete().Model((*enrollmentModels.RequestChildOffering)(nil)).
		ModelTableExpr(`enrollment.request_child_offerings AS "request_child_offering"`).
		Where("request_child_id = ?", fixture.child.ID).
		Where("care_offering_id = ?", fixture.mittag.ID).
		Exec(t.Context())
	require.NoError(t, err)
	manualTime, err := time.Parse("2006-01-02 15:04", "2000-01-01 13:15")
	require.NoError(t, err)
	existingNote := "Bestehende Busnotiz"
	manual := &scheduleModels.StudentPickupSchedule{
		StudentID: student.ID, Weekday: 5, PickupTime: manualTime, Notes: &existingNote, CreatedBy: staff.StaffID,
	}
	manual.SetTenantID(student.TenantID)
	require.NoError(t, repositories.NewFactory(tc.db).StudentPickupSchedule.UpsertSchedule(testpkg.Ctx(t), manual))

	effectiveFrom := timezone.TodayDate().String()
	schedules := make([]map[string]any, 0, 5)
	careDays := make([]int, 0, 5)
	for weekday := 1; weekday <= 5; weekday++ {
		row := map[string]any{"weekday": weekday, "pickup_time": "14:30"}
		if weekday == scheduleModels.WeekdayFriday {
			row["notes"] = "Fährt mit dem Bus"
		}
		schedules = append(schedules, row)
		careDays = append(careDays, weekday)
	}
	baseBody := map[string]any{
		"schedules": schedules, "care_days": careDays, "effective_from": effectiveFrom,
	}
	preview := postPickupAdjustmentPreview(t, tc, student.ID, account.ID, baseBody)
	require.Len(t, preview.MatchingOfferings, 1)
	assert.Equal(t, fmt.Sprint(fixture.mittag.ID), preview.MatchingOfferings[0].OfferingID)
	offeringBody := cloneMap(baseBody)
	offeringBody["selections"] = []map[string]any{{
		"offering_id":   fmt.Sprint(fixture.mittag.ID),
		"selected_days": []string{},
	}}
	offeringBody["arrival_schedules"] = fiveDayArrivalBody("08:20", "Haupteingang")
	previewWithNewNote := postPickupAdjustmentPreview(t, tc, student.ID, account.ID, offeringBody)
	require.Len(t, previewWithNewNote.RemovedManualNotes, 1)
	assert.Equal(t, scheduleModels.WeekdayFriday, previewWithNewNote.RemovedManualNotes[0].Weekday)
	assert.Equal(t, existingNote, previewWithNewNote.RemovedManualNotes[0].Note)

	exceptionBody := cloneMap(baseBody)
	exceptionBody["preview_token"] = preview.PreviewToken
	exceptionBody["resolution"] = "exception"
	exceptionReq := testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/%d/pickup-schedules/apply", student.ID), exceptionBody,
	)
	exceptionRec := authExec(
		t, tc, exceptionReq, testutil.AdminTestClaims(int(account.ID)), []string{"users:update"},
	)
	require.Equal(t, http.StatusOK, exceptionRec.Code, exceptionRec.Body.String())
	links, err := tc.services.EnrollmentDecision.ListChildOfferings(t.Context(), fixture.child.RequestID)
	require.NoError(t, err)
	require.Len(t, links[fixture.child.ID].Current, 1)
	assert.Equal(t, fixture.ganztag.ID, links[fixture.child.ID].Current[0].OfferingID,
		"an explicit exception must keep the current offering")

	confirmedPreview := postPickupAdjustmentPreview(t, tc, student.ID, account.ID, offeringBody)
	require.NotEmpty(t, confirmedPreview.PreviewToken)
	require.Len(t, confirmedPreview.RemovedManualNotes, 1)
	assert.Equal(t, scheduleModels.WeekdayFriday, confirmedPreview.RemovedManualNotes[0].Weekday)
	assert.Equal(t, "Fährt mit dem Bus", confirmedPreview.RemovedManualNotes[0].Note)

	futureBody := cloneMap(offeringBody)
	futureDate := timezone.TodayDate().AddDays(7)
	futureBody["effective_from"] = futureDate.String()
	futurePreviewReq := testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/%d/pickup-schedules/preview", student.ID), futureBody,
	)
	futurePreviewRec := authExec(
		t, tc, futurePreviewReq, testutil.AdminTestClaims(int(account.ID)), []string{"users:update"},
	)
	assert.Equal(t, http.StatusConflict, futurePreviewRec.Code, futurePreviewRec.Body.String())
	assert.Contains(t, futurePreviewRec.Body.String(), `"code":"pickup.future_manual_reset"`)
	futureApplyBody := cloneMap(futureBody)
	futureApplyBody["preview_token"] = confirmedPreview.PreviewToken
	futureApplyBody["resolution"] = "offering"
	futureReq := testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/%d/pickup-schedules/apply", student.ID), futureApplyBody,
	)
	futureRec := authExec(
		t, tc, futureReq, testutil.AdminTestClaims(int(account.ID)), []string{"users:update"},
	)
	assert.Equal(t, http.StatusConflict, futureRec.Code, futureRec.Body.String())
	assert.Contains(t, futureRec.Body.String(), `"code":"pickup.future_manual_reset"`)
	futureLinks, err := repositories.NewFactory(tc.db).RequestChildOffering.
		ListByRequestChildIDAtDate(testpkg.Ctx(t), fixture.child.ID, futureDate)
	require.NoError(t, err)
	require.Len(t, futureLinks, 1)
	assert.Equal(t, fixture.ganztag.ID, futureLinks[0].CareOfferingID,
		"the rejected future change must not alter bookings")

	_, err = tc.db.NewUpdate().Model(fixture.mittag).
		ModelTableExpr(`enrollment.care_offerings AS "care_offering"`).
		Set("capacity = 0").
		WherePK().Exec(t.Context())
	require.NoError(t, err)
	fullBody := cloneMap(offeringBody)
	fullBody["preview_token"] = confirmedPreview.PreviewToken
	fullBody["resolution"] = "offering"
	fullReq := testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/%d/pickup-schedules/apply", student.ID), fullBody,
	)
	fullRec := authExec(
		t, tc, fullReq, testutil.AdminTestClaims(int(account.ID)), []string{"users:update"},
	)
	assert.Equal(t, http.StatusConflict, fullRec.Code, fullRec.Body.String())
	assert.Contains(t, fullRec.Body.String(), `"code":"pickup.offering_capacity_full"`)
	links, err = tc.services.EnrollmentDecision.ListChildOfferings(t.Context(), fixture.child.RequestID)
	require.NoError(t, err)
	require.Len(t, links[fixture.child.ID].Current, 1)
	assert.Equal(t, fixture.ganztag.ID, links[fixture.child.ID].Current[0].OfferingID,
		"a capacity conflict must not alter bookings")
	_, err = tc.db.NewUpdate().Model(fixture.mittag).
		ModelTableExpr(`enrollment.care_offerings AS "care_offering"`).
		Set("capacity = NULL").
		WherePK().Exec(t.Context())
	require.NoError(t, err)
	confirmedPreview = postPickupAdjustmentPreview(t, tc, student.ID, account.ID, offeringBody)
	concurrentArrivalTime, err := time.Parse("2006-01-02 15:04", "2000-01-01 07:55")
	require.NoError(t, err)
	concurrentArrival := &scheduleModels.StudentArrivalSchedule{
		StudentID: student.ID, Weekday: scheduleModels.WeekdayMonday,
		ExpectedArrival: concurrentArrivalTime, CreatedBy: staff.StaffID,
	}
	concurrentArrival.SetTenantID(student.TenantID)
	require.NoError(t, repositories.NewFactory(tc.db).StudentArrivalSchedule.UpsertSchedule(
		testpkg.Ctx(t), concurrentArrival,
	))
	staleArrivalBody := cloneMap(offeringBody)
	staleArrivalBody["preview_token"] = confirmedPreview.PreviewToken
	staleArrivalBody["resolution"] = "offering"
	staleArrivalReq := testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/%d/pickup-schedules/apply", student.ID), staleArrivalBody,
	)
	staleArrivalRec := authExec(
		t, tc, staleArrivalReq, testutil.AdminTestClaims(int(account.ID)), []string{"users:update"},
	)
	assert.Equal(t, http.StatusConflict, staleArrivalRec.Code, staleArrivalRec.Body.String())
	assert.Contains(t, staleArrivalRec.Body.String(), `"code":"pickup.preview_stale"`)
	confirmedPreview = postPickupAdjustmentPreview(t, tc, student.ID, account.ID, offeringBody)
	arrivalRowsBefore, err := repositories.NewFactory(tc.db).StudentArrivalSchedule.FindByStudentID(
		testpkg.Ctx(t), student.ID,
	)
	require.NoError(t, err)

	applyBody := cloneMap(offeringBody)
	applyBody["preview_token"] = confirmedPreview.PreviewToken
	applyBody["resolution"] = "offering"
	applyReq := testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/%d/pickup-schedules/apply", student.ID), applyBody,
	)
	applyRec := authExec(
		t, tc, applyReq, testutil.AdminTestClaims(int(account.ID)), []string{"users:update"},
	)
	require.Equal(t, http.StatusOK, applyRec.Code, applyRec.Body.String())

	links, err = tc.services.EnrollmentDecision.ListChildOfferings(t.Context(), fixture.child.RequestID)
	require.NoError(t, err)
	require.Len(t, links[fixture.child.ID].Current, 1)
	assert.Equal(t, fixture.mittag.ID, links[fixture.child.ID].Current[0].OfferingID)
	var manualRows []*scheduleModels.StudentPickupSchedule
	require.NoError(t, tc.db.NewSelect().Model(&manualRows).
		ModelTableExpr(`schedule.student_pickup_schedules AS "student_pickup_schedule"`).
		Where("student_id = ?", student.ID).
		Where("source <> ?", scheduleModels.PickupScheduleSourceCareOffering).
		Scan(t.Context()))
	assert.Empty(t, manualRows, "the selected offering must replace every lasting manual pickup time")
	arrivalRows, err := repositories.NewFactory(tc.db).StudentArrivalSchedule.FindByStudentID(testpkg.Ctx(t), student.ID)
	require.NoError(t, err)
	assert.Equal(t, arrivalRowsBefore, arrivalRows,
		"an offering change must leave the existing arrival plan unchanged")

	history, err := tc.services.StudentAudit.GetChangeHistory(testpkg.Ctx(t), student.ID)
	require.NoError(t, err)
	require.NotEmpty(t, history)
	assert.Contains(t, valueOrEmpty(history[0].NewValue), "Angebot geändert")
}

func TestPickupAdjustmentProtectedRouterRollsBackKnownErrorAfterOfferingWrite(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	student := testpkg.CreateTestStudent(t, tc.db, "PickupRollback", "Child", "PR1")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "PickupRollback", "Staff")
	fixture := setupCorrectionFixture(t, tc, student.ID, student.TenantID, "Child")
	require.NoError(t, tc.services.Settings.SetValue(
		testpkg.Ctx(t), configModels.KeyRequirePickupOfferingReview, true, nil, nil,
	))
	for offering, pickupTime := range map[*enrollmentModels.CareOffering]string{
		fixture.ganztag: "16:00", fixture.mittag: "14:30",
	} {
		_, err := tc.db.NewUpdate().Model(offering).
			ModelTableExpr(`enrollment.care_offerings AS "care_offering"`).
			Set("pickup_times = ?", map[string]string{
				"mon": pickupTime, "tue": pickupTime, "wed": pickupTime, "thu": pickupTime, "fri": pickupTime,
			}).WherePK().Exec(t.Context())
		require.NoError(t, err)
	}
	_, err := tc.db.NewDelete().Model((*enrollmentModels.RequestChildOffering)(nil)).
		ModelTableExpr(`enrollment.request_child_offerings AS "request_child_offering"`).
		Where("request_child_id = ?", fixture.child.ID).
		Where("care_offering_id = ?", fixture.mittag.ID).Exec(t.Context())
	require.NoError(t, err)

	body := pickupAdjustmentFiveDayBody(fixture.mittag.ID)
	preview := postPickupAdjustmentPreview(t, tc, student.ID, account.ID, body)
	body["preview_token"] = preview.PreviewToken
	body["resolution"] = "offering"
	realCoordinator, ok := tc.services.OfferingChanges.(enrollmentService.DirectOfferingAdjustmentCoordinator)
	require.True(t, ok)
	tc.resource.PickupAdjustmentService = pickupAdjustmentServiceWithCoordinator(
		tc, failAfterOfferingWriteCoordinator{realCoordinator},
	)

	req := testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/%d/pickup-schedules/apply", student.ID), body,
	)
	rec := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"users:update"})
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"pickup.invalid"`)
	links, err := tc.services.EnrollmentDecision.ListChildOfferings(t.Context(), fixture.child.RequestID)
	require.NoError(t, err)
	require.Len(t, links[fixture.child.ID].Current, 1)
	assert.Equal(t, fixture.ganztag.ID, links[fixture.child.ID].Current[0].OfferingID)
	adjustments, err := tc.db.NewSelect().TableExpr("audit.enrollment_offering_adjustments").
		Where("request_child_id = ?", fixture.child.ID).Count(t.Context())
	require.NoError(t, err)
	assert.Zero(t, adjustments, "the failed 400 response must roll back its offering audit")

}

func TestBulkPickupAdjustmentRequiresAndAuditsExplicitExceptions(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	first := testpkg.CreateTestStudent(t, tc.db, "BulkPickupReview1", "Child", "BPR1")
	second := testpkg.CreateTestStudent(t, tc.db, "BulkPickupReview2", "Child", "BPR2")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "BulkPickupReview", "Staff")
	require.NoError(t, tc.services.Settings.SetValue(
		testpkg.Ctx(t), configModels.KeyRequirePickupOfferingReview, true, nil, nil,
	))
	body := map[string]any{
		"student_ids": []int64{first.ID, second.ID},
		"schedules":   []map[string]any{{"weekday": 1, "pickup_time": "16:20"}},
	}

	req := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/pickup-schedules/bulk", body)
	rec := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"users:update"})
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"pickup.bulk_exception_confirmation_required"`)

	body["confirmed_exception"] = true
	confirmedReq := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/pickup-schedules/bulk", body)
	confirmedRec := authExec(
		t, tc, confirmedReq, testutil.AdminTestClaims(int(account.ID)), []string{"users:update"},
	)
	require.Equal(t, http.StatusOK, confirmedRec.Code, confirmedRec.Body.String())

	for _, studentID := range []int64{first.ID, second.ID} {
		history, err := tc.services.StudentAudit.GetChangeHistory(testpkg.Ctx(t), studentID)
		require.NoError(t, err)
		require.NotEmpty(t, history)
		assert.Equal(t, auditModels.StudentFieldPickupSchedule, history[0].FieldName)
		assert.Contains(t, valueOrEmpty(history[0].OldValue), "Kein Wochenplan")
		assert.Contains(t, valueOrEmpty(history[0].NewValue), "Mo 16:20 Uhr")
	}
}

func pickupAdjustmentFiveDayBody(offeringID int64) map[string]any {
	schedules := make([]map[string]any, 0, 5)
	careDays := make([]int, 0, 5)
	for weekday := 1; weekday <= 5; weekday++ {
		schedules = append(schedules, map[string]any{"weekday": weekday, "pickup_time": "14:30"})
		careDays = append(careDays, weekday)
	}
	return map[string]any{
		"schedules": schedules, "care_days": careDays, "effective_from": timezone.TodayDate().String(),
		"selections": []map[string]any{{"offering_id": fmt.Sprint(offeringID), "selected_days": []string{}}},
	}
}

func fiveDayArrivalBody(arrivalTime, mondayNote string) []map[string]any {
	rows := make([]map[string]any, 0, 5)
	for weekday := scheduleModels.WeekdayMonday; weekday <= scheduleModels.WeekdayFriday; weekday++ {
		row := map[string]any{"weekday": weekday, "expected_arrival": arrivalTime}
		if weekday == scheduleModels.WeekdayMonday && mondayNote != "" {
			row["notes"] = mondayNote
		}
		rows = append(rows, row)
	}
	return rows
}

func pickupAdjustmentServiceWithCoordinator(
	tc *testContext,
	coordinator enrollmentService.DirectOfferingAdjustmentCoordinator,
) enrollmentService.PickupAdjustmentService {
	repos := repositories.NewFactory(tc.db)
	baselines := scheduleService.NewPickupBaselineServiceWithSettings(
		repos.StudentPickupSchedule, repos.RequestChildOffering, repos.CareOffering, tc.services.Settings,
	)
	return enrollmentService.NewPickupAdjustmentService(enrollmentService.PickupAdjustmentServiceConfig{
		PickupSchedules: tc.services.PickupSchedule, ArrivalSchedules: tc.services.ArrivalSchedule,
		PickupScheduleRepo:  repos.StudentPickupSchedule,
		ArrivalScheduleRepo: repos.StudentArrivalSchedule,
		PickupBaselines:     baselines, Offerings: coordinator, Settings: tc.services.Settings,
		Audit: tc.services.StudentAudit, Students: repos.Student, DB: tc.db,
	})
}

type pickupAdjustmentPreviewEnvelopeData struct {
	PreviewToken      string `json:"preview_token"`
	MatchingOfferings []struct {
		OfferingID string `json:"offering_id"`
	} `json:"matching_offerings"`
	RemovedManualNotes []struct {
		Weekday int    `json:"weekday"`
		Note    string `json:"note"`
	} `json:"removed_manual_notes"`
}

func postPickupAdjustmentPreview(
	t *testing.T,
	tc *testContext,
	studentID, accountID int64,
	body map[string]any,
) pickupAdjustmentPreviewEnvelopeData {
	t.Helper()
	req := testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/%d/pickup-schedules/preview", studentID), body,
	)
	rec := authExec(t, tc, req, testutil.AdminTestClaims(int(accountID)), []string{"users:update"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var envelope struct {
		Data pickupAdjustmentPreviewEnvelopeData `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	return envelope.Data
}

func cloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source)+2)
	for key, value := range source {
		result[key] = value
	}
	return result
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
