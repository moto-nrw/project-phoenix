package care

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-ogs/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// SickNoteResult is the outcome of a parent absence submission: StatusDays for
// the days written directly, PendingRequest for the days the school decides,
// either because the absence type requires office approval (#1845, #2447,
// #2449) or because the school already entered something for those days
// (#5698). A report over free and school days carries both.
type SickNoteResult struct {
	StatusDays     []*absencerecords.StudentStatusDay
	PendingRequest *ExcusedAbsenceRequest
}

// SubmitSickNote reports the child absent for the given dates with the chosen
// status. The status is either StudentStatusDaySick (a "Krankmeldung": flips the
// live sick flag when today is included) or StudentStatusDayExcused (an
// "entschuldigte Abmeldung": stored with NO live flag, per issue #1735). A note
// is mandatory for both absence types.
//
// Each absence type has an independent approval setting. When its gate is on,
// the report creates a PENDING request and writes no status day until staff
// approve it. With the gate off, the report is applied directly.
func (s *Service) SubmitSickNote(ctx context.Context, accountID, studentID int64, dates []calendar.Date, reason, status string, recipientGuardianProfileIDs []int64) (*SickNoteResult, error) {
	if len(dates) == 0 {
		return nil, ErrNoDates
	}
	if status != absencerecords.StudentStatusDaySick && status != absencerecords.StudentStatusDayExcused {
		return nil, ErrInvalidStatus
	}

	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, GuardianPermissionSickNoteSubmit)
	if err != nil {
		return nil, err
	}
	// A child whose care at this school has ended keeps read access to
	// what happened, but nothing new can be submitted for them (#2487).
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}
	if err := s.requireAbsenceReportsEnabled(ctx, child.TenantID, status); err != nil {
		return nil, err
	}
	note, err := s.absenceNote(ctx, child.TenantID, reason)
	if err != nil {
		return nil, err
	}

	approvalKey := KeyParentSickRequiresApproval
	if status == absencerecords.StudentStatusDayExcused {
		approvalKey = KeyParentExcusedRequiresApproval
	}
	requiresApproval, err := s.Settings.ResolveBoolForTenant(ctx, child.TenantID, approvalKey)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve absence-approval setting %s: %w", approvalKey, err)
	}
	if requiresApproval {
		return s.submitAbsenceRequest(ctx, child, accountID, studentID, dates, note, status, recipientGuardianProfileIDs)
	}
	return s.reportAbsence(ctx, child, absenceReport{
		accountID: accountID, studentID: studentID, dates: dates, note: note, status: status,
		recipientGuardianProfileIDs: recipientGuardianProfileIDs,
	})
}

// requireAbsenceReportsEnabled checks the school's sick-note switch and the
// switch of the chosen absence type.
func (s *Service) requireAbsenceReportsEnabled(ctx context.Context, tenantID int64, status string) error {
	enabled, err := s.Settings.ResolveBoolForTenant(ctx, tenantID, KeyParentSickNoteEnabled)
	if err != nil {
		return fmt.Errorf("parent: resolve sick-note setting: %w", err)
	}
	if !enabled {
		return ErrSickNoteDisabled
	}
	reportKey := KeyParentSickReportsEnabled
	if status == absencerecords.StudentStatusDayExcused {
		reportKey = KeyParentExcusedReportsEnabled
	}
	enabled, err = s.Settings.ResolveBoolForTenant(ctx, tenantID, reportKey)
	if err != nil {
		return fmt.Errorf("parent: resolve report setting %s: %w", reportKey, err)
	}
	if !enabled {
		return ErrSickNoteDisabled
	}
	return nil
}

// absenceNote trims the family's note and enforces its length and the
// school's reason policy.
func (s *Service) absenceNote(ctx context.Context, tenantID int64, reason string) (string, error) {
	// Count characters (runes), not UTF-8 bytes, so the limit matches the
	// frontend's maxLength — a German text with umlauts stays under the budget.
	trimmedNote := strings.TrimSpace(reason)
	if utf8.RuneCountInString(trimmedNote) > MaxParentNoteLen {
		return "", ErrNoteTooLong
	}
	// The note is mandatory only while the school's reason policy asks the
	// family for one (#2267, story 28). Every other school keeps the previous
	// behaviour, including one that never configured the setting.
	if trimmedNote == "" && s.guardianReasonRequired(ctx, tenantID) {
		return "", ErrEmptyNote
	}
	return trimmedNote, nil
}

// sickNoteWriteAccess is what an absence report and its request hold until
// they commit.
var sickNoteWriteAccess = childWriteNeeds(GuardianPermissionSickNoteSubmit)

// absenceReport is one direct parent absence submission.
type absenceReport struct {
	accountID int64
	studentID int64
	dates     []calendar.Date
	note      string
	status    string
	// recipientGuardianProfileIDs share a request for school days (#5698).
	recipientGuardianProfileIDs []int64
}

// reportAbsence applies a direct absence in one tenant unit of work: Care Plan
// records the status days, People Directory the live flags for today. Days the
// school already holds an entry for become one request the school decides
// (#5698): families never overwrite a school entry.
func (s *Service) reportAbsence(ctx context.Context, child *Child, report absenceReport) (*SickNoteResult, error) {
	if s.GuardianAbsences == nil {
		return nil, errors.New("parent: guardian absence command is not configured")
	}
	var notePtr *string
	if report.note != "" {
		notePtr = &report.note
	}
	result := &SickNoteResult{}
	var freeDays int
	txErr := s.WithChildWrite(ctx, report.accountID, report.studentID, child, sickNoteWriteAccess, func(txCtx context.Context) error {
		split, err := s.splitSchoolDays(txCtx, report)
		if err != nil {
			return err
		}
		freeDays = len(split.free)
		if len(split.free) > 0 {
			direct := report
			direct.dates = split.free
			if result.StatusDays, err = s.writeAbsence(txCtx, child.TenantID, split.student, direct, notePtr); err != nil {
				return err
			}
		}
		if len(split.school) > 0 {
			result.PendingRequest, err = s.createAbsenceRequest(txCtx, child, report.accountID, report.studentID,
				split.school, report.note, report.status, report.recipientGuardianProfileIDs)
			return err
		}
		return nil
	})
	if txErr != nil {
		// Translates the request refusals; every other error stays wrapped.
		return nil, mapExcusedRequestError(txErr, "submit sick note")
	}

	s.Logger.Info("parent submitted absence",
		slog.Int64("account_id", report.accountID),
		slog.Int64("student_id", report.studentID),
		slog.Int64("tenant_id", child.TenantID),
		slog.String("status", report.status),
		slog.Int("days", freeDays),
		slog.Int("school_days", len(report.dates)-freeDays),
		slog.Bool("has_reason", notePtr != nil),
	)
	return result, nil
}

// schoolDaySplit separates the reported days into the ones the family writes
// directly and the ones the school already holds an entry for.
type schoolDaySplit struct {
	student *PortalStudent
	free    []calendar.Date
	school  []calendar.Date
}

// splitSchoolDays reads the child's entries under the student row lock that
// serializes every staff status and partial-absence write, so the split
// cannot race a school entry. A school entry is a staff-entered partial
// absence or a full-day status the school set (any source but the family).
func (s *Service) splitSchoolDays(ctx context.Context, report absenceReport) (schoolDaySplit, error) {
	fresh, err := s.StudentRepo.FindByIDForUpdate(ctx, report.studentID)
	if err != nil {
		return schoolDaySplit{}, err
	}
	// ResolvePermittedChild ran before this transaction. Re-check the
	// interval after acquiring the same row lock as care exits so a care exit
	// cannot commit between authorization and this write.
	if fresh.CareEndedOn(s.todayDate()) {
		return schoolDaySplit{}, ErrChildCareEnded
	}
	school, err := s.schoolEntryDays(ctx, report.studentID, report.dates)
	if err != nil {
		return schoolDaySplit{}, err
	}
	split := schoolDaySplit{student: fresh}
	for _, date := range report.dates {
		if _, held := school[date]; held {
			split.school = append(split.school, date)
		} else {
			split.free = append(split.free, date)
		}
	}
	return split, nil
}

func (s *Service) schoolEntryDays(ctx context.Context, studentID int64, dates []calendar.Date) (map[calendar.Date]struct{}, error) {
	from := slices.MinFunc(dates, calendar.Date.Compare)
	to := slices.MaxFunc(dates, calendar.Date.Compare)
	held := make(map[calendar.Date]struct{})
	pickups, err := s.CareExceptions.ListPickupExceptions(ctx, StudentScheduleFilter{StudentIDs: []int64{studentID}, From: from, To: to})
	if err != nil {
		return nil, err
	}
	for _, row := range pickups {
		if row.HasManualPartialAbsence() {
			held[row.ExceptionDate] = struct{}{}
		}
	}
	days, err := s.StatusDayRepo.FindActiveByStudentAndDateRange(ctx, studentID, from, to)
	if err != nil {
		return nil, err
	}
	for _, day := range days {
		if day.Source != absencerecords.StudentStatusSourceParent {
			held[calendar.Date(day.Date)] = struct{}{}
		}
	}
	for date := range held {
		if !slices.Contains(dates, date) {
			delete(held, date)
		}
	}
	return held, nil
}

// writeAbsence records the free days. fresh is the student row the caller
// locked: that lock serializes every parent status write with staff writes on
// the same student, including future-only ranges, and staff conflict
// detection relies on it to make its read and write one atomic decision.
func (s *Service) writeAbsence(ctx context.Context, tenantID int64, fresh *PortalStudent, report absenceReport, note *string) ([]*absencerecords.StudentStatusDay, error) {
	now := time.Now()
	today := s.todayDate()
	includesToday := slices.Contains(report.dates, today)
	notifyAbsence := includesToday && isNewParentReportableAbsence(fresh, report.status)

	// Care Plan takes the care-day locks, refuses a day with a manual partial
	// absence, clears the other statuses and records the reported ones.
	days, err := s.GuardianAbsences.ReportGuardianAbsence(ctx, GuardianAbsenceReport{
		StudentID: report.studentID, GuardianAccountID: report.accountID,
		Dates: carePlanDates(report.dates), Status: report.status, Note: note, ReportedAt: now,
	})
	if errors.Is(err, s.CarePlanPolicy.Error(CarePlanManualPartialAbsenceConflict)) ||
		errors.Is(err, s.CarePlanPolicy.Error(CarePlanSchoolStatusDayConflict)) {
		return nil, ErrCareExceptionConflict
	}
	if err != nil {
		return nil, err
	}
	if includesToday {
		applyLiveStatusForParentToday(fresh, report.status, now)
		if err := s.Students.SetStudentLiveAbsence(ctx, studentLiveAbsence(fresh)); err != nil {
			return nil, err
		}
	}
	rows := statusDaysFromCarePlan(days)
	if notifyAbsence && s.AbsenceNotifier != nil {
		if err := s.AbsenceNotifier.NotifyAbsenceReported(ctx, AbsenceReport{
			TenantID: tenantID, StudentIDs: []int64{report.studentID}, Status: report.status, Dates: report.dates,
			FromParent: true, ActorAccountID: report.accountID,
		}); err != nil {
			return nil, err
		}
	}
	if err := s.afterAbsenceCommit(ctx, tenantID, report, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// afterAbsenceCommit posts the chat pill and wakes the live views once the
// absence has committed.
func (s *Service) afterAbsenceCommit(ctx context.Context, tenantID int64, report absenceReport, rows []*absencerecords.StudentStatusDay) error {
	pillBody := SickNoteEventBody(report.status, report.dates)
	pillRefID := firstStatusID(rows)
	if err := s.SelfService.EmitSelfServicePill(ctx, tenantID, report.studentID, report.accountID, "sick_note", pillBody, "active.student_status_days", pillRefID); err != nil {
		return err
	}
	s.Runtime.RegisterAfterCommit(ctx, func() {
		s.broadcastStudentUpdated(tenantID, report.studentID)
		// broadcastStudentUpdated is staff-only and emitSelfServicePill wakes
		// just the acting guardian's thread; fan out to EVERY guardian so a
		// co-guardian's open tab drops the stale presence too (#1725 review).
		s.SelfService.WakeChildGuardians(tenantID, report.studentID)
	})
	return nil
}

// statusDaysFromCarePlan returns Care Plan's status days in the portal's
// response shape.
func statusDaysFromCarePlan(days []StudentStatusDay) []*absencerecords.StudentStatusDay {
	rows := make([]*absencerecords.StudentStatusDay, 0, len(days))
	for _, day := range days {
		row := &absencerecords.StudentStatusDay{
			StudentID: day.StudentID, Date: absencerecords.Date(day.Date), Status: day.Status,
			ReportedAt: day.ReportedAt, ClearedAt: day.ClearedAt, Source: day.Source,
			GuardianAccountID: day.GuardianAccountID, Note: day.Note,
		}
		row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = day.ID, day.TenantID, day.CreatedAt, day.UpdatedAt
		rows = append(rows, row)
	}
	return rows
}

func isNewParentReportableAbsence(student *PortalStudent, status string) bool {
	switch status {
	case absencerecords.StudentStatusDaySick:
		return student.Sick == nil || !*student.Sick
	case absencerecords.StudentStatusDayExcused:
		return student.Excused == nil || !*student.Excused
	default:
		return false
	}
}

// submitAbsenceRequest turns a sick or excused report into a pending office
// request inside the child's tenant transaction.
func (s *Service) submitAbsenceRequest(ctx context.Context, child *Child, accountID, studentID int64, dates []calendar.Date, note, status string, recipientGuardianProfileIDs []int64) (*SickNoteResult, error) {
	var req *ExcusedAbsenceRequest
	txErr := s.WithChildWrite(ctx, accountID, studentID, child, sickNoteWriteAccess, func(txCtx context.Context) error {
		// The initial authorization snapshot may predate a concurrent care exit.
		// Lock and re-read the child before creating a pending request so both
		// absence paths obey the same read-only boundary.
		fresh, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
		if err != nil {
			return err
		}
		if fresh.CareEndedOn(s.todayDate()) {
			return ErrChildCareEnded
		}
		req, err = s.createAbsenceRequest(txCtx, child, accountID, studentID, dates, note, status, recipientGuardianProfileIDs)
		return err
	})
	if txErr != nil {
		return nil, mapExcusedRequestError(txErr, "submit absence request")
	}
	return &SickNoteResult{PendingRequest: req}, nil
}

// createAbsenceRequest files the pending request and its recipient choice in
// the caller's tenant transaction; the caller holds the student row lock.
func (s *Service) createAbsenceRequest(ctx context.Context, child *Child, accountID, studentID int64, dates []calendar.Date, note, status string, recipientGuardianProfileIDs []int64) (*ExcusedAbsenceRequest, error) {
	if s.ExcusedRequests == nil {
		return nil, fmt.Errorf("parent: absence request service not configured")
	}
	// The note is mandatory only while the school's reason policy asks the
	// family for one (#2267, story 28).
	created, err := s.ExcusedRequests.Submit(ctx, ExcusedRequestCreateInput{
		StudentID:         studentID,
		GuardianAccountID: accountID,
		Dates:             carePlanDates(dates),
		Note:              note,
		AbsenceStatus:     status,
		NoteRequired:      s.guardianReasonRequired(ctx, child.TenantID),
	})
	if err != nil {
		return nil, err
	}
	// The recipient choice is written in the SAME transaction as the
	// request: a refused share rolls the request back with it, so a family
	// never ends up with a request nobody they picked can see (#2267).
	if err := s.RequestSharing.ShareRequestInTx(
		ctx, accountID, studentID, RequestShareExcused, created.ID, recipientGuardianProfileIDs,
	); err != nil {
		return nil, err
	}
	s.Logger.Info("parent submitted absence request",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.TenantID),
		slog.Int64("request_id", created.ID),
		slog.String("status", status),
		slog.Int("days", len(dates)),
	)
	return created, nil
}

// applyLiveStatusForParentToday updates the live student flags for a parent
// submission that includes today. A "Krankmeldung" (sick) flips the live sick
// flag on and clears any excused flag, exactly as before. A "Termin/Abwesenheit"
// (excused) sets NO live flag per issue #1735 — it only clears a stale live sick
// flag so the row stays consistent with the now-cleared sick status day, and
// leaves a staff-set excused flag untouched.
func applyLiveStatusForParentToday(student *PortalStudent, status string, now time.Time) {
	trueVal := true
	falseVal := false
	switch status {
	case absencerecords.StudentStatusDaySick:
		student.Sick = &trueVal
		student.SickSince = &now
		student.Excused = &falseVal
		student.ExcusedSince = nil
	case absencerecords.StudentStatusDayExcused:
		student.Sick = &falseVal
		student.SickSince = nil
	}
}

// studentLiveAbsence is the child's live absence flags after the change.
func studentLiveAbsence(student *PortalStudent) StudentLiveAbsence {
	return StudentLiveAbsence{
		StudentID: student.ID, Sick: student.Sick, SickSince: student.SickSince,
		Excused: student.Excused, ExcusedSince: student.ExcusedSince,
	}
}

func firstStatusID(rows []*absencerecords.StudentStatusDay) *int64 {
	for _, row := range rows {
		if row != nil && row.ID > 0 {
			id := row.ID
			return &id
		}
	}
	return nil
}
