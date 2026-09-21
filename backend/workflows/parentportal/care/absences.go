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

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	notificationsSvc "github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// SickNoteResult is the outcome of a parent absence submission. Exactly one of
// its fields is populated: StatusDays for a direct write, PendingRequest when
// the selected absence type requires office approval (#1845, #2447, #2449).
type SickNoteResult struct {
	StatusDays     []*absencerecords.StudentStatusDay
	PendingRequest *careplan.ExcusedAbsenceRequest
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
func (s *Service) SubmitSickNote(ctx context.Context, accountID, studentID int64, dates []timezone.Date, reason, status string, recipientGuardianProfileIDs []int64) (*SickNoteResult, error) {
	if len(dates) == 0 {
		return nil, ErrNoDates
	}
	if status != absencerecords.StudentStatusDaySick && status != absencerecords.StudentStatusDayExcused {
		return nil, ErrInvalidStatus
	}

	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionSickNoteSubmit)
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

	approvalKey := configModels.KeyParentSickRequiresApproval
	if status == absencerecords.StudentStatusDayExcused {
		approvalKey = configModels.KeyParentExcusedRequiresApproval
	}
	requiresApproval, err := s.Settings.ResolveBoolForTenant(ctx, child.TenantID, approvalKey)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve absence-approval setting %s: %w", approvalKey, err)
	}
	if requiresApproval {
		return s.submitAbsenceRequest(ctx, child, accountID, studentID, dates, note, status, recipientGuardianProfileIDs)
	}
	return s.reportAbsence(ctx, child, absenceReport{accountID: accountID, studentID: studentID, dates: dates, note: note, status: status})
}

// requireAbsenceReportsEnabled checks the school's sick-note switch and the
// switch of the chosen absence type.
func (s *Service) requireAbsenceReportsEnabled(ctx context.Context, tenantID int64, status string) error {
	enabled, err := s.Settings.ResolveBoolForTenant(ctx, tenantID, configModels.KeyParentSickNoteEnabled)
	if err != nil {
		return fmt.Errorf("parent: resolve sick-note setting: %w", err)
	}
	if !enabled {
		return ErrSickNoteDisabled
	}
	reportKey := configModels.KeyParentSickReportsEnabled
	if status == absencerecords.StudentStatusDayExcused {
		reportKey = configModels.KeyParentExcusedReportsEnabled
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

// absenceReport is one direct parent absence submission.
type absenceReport struct {
	accountID int64
	studentID int64
	dates     []timezone.Date
	note      string
	status    string
}

// reportAbsence applies a direct absence in one tenant unit of work: Care Plan
// records the status days, People Directory the live flags for today.
func (s *Service) reportAbsence(ctx context.Context, child *Child, report absenceReport) (*SickNoteResult, error) {
	if s.GuardianAbsences == nil {
		return nil, errors.New("parent: guardian absence command is not configured")
	}
	var notePtr *string
	if report.note != "" {
		notePtr = &report.note
	}
	var result []*absencerecords.StudentStatusDay
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		rows, err := s.writeAbsence(txCtx, child.TenantID, report, notePtr)
		result = rows
		return err
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: submit sick note: %w", txErr)
	}

	s.Logger.Info("parent submitted absence",
		slog.Int64("account_id", report.accountID),
		slog.Int64("student_id", report.studentID),
		slog.Int64("tenant_id", child.TenantID),
		slog.String("status", report.status),
		slog.Int("days", len(report.dates)),
		slog.Bool("has_reason", notePtr != nil),
	)
	return &SickNoteResult{StatusDays: result}, nil
}

func (s *Service) writeAbsence(ctx context.Context, tenantID int64, report absenceReport, note *string) ([]*absencerecords.StudentStatusDay, error) {
	now := time.Now()
	today := s.todayDate()
	// Serialize every parent status write with staff writes on the same
	// student, including future-only ranges. Staff conflict detection relies
	// on this lock to make its read and write one atomic decision.
	fresh, err := s.StudentRepo.FindByIDForUpdate(ctx, report.studentID)
	if err != nil {
		return nil, err
	}
	// ResolvePermittedChild ran before this transaction. Re-check the
	// interval after acquiring the same row lock as care exits so a care exit
	// cannot commit between authorization and this write.
	if fresh.CareEndedOn(today) {
		return nil, ErrChildCareEnded
	}
	includesToday := slices.Contains(report.dates, today)
	notifyAbsence := includesToday && isNewParentReportableAbsence(fresh, report.status)

	// Care Plan takes the care-day locks, refuses a day with a manual partial
	// absence, clears the other statuses and records the reported ones.
	days, err := s.GuardianAbsences.ReportGuardianAbsence(ctx, careplan.GuardianAbsenceReport{
		StudentID: report.studentID, GuardianAccountID: report.accountID,
		Dates: carePlanDates(report.dates), Status: report.status, Note: note, ReportedAt: now,
	})
	if errors.Is(err, careplan.ErrManualPartialAbsenceConflict) {
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
		if err := s.AbsenceNotifier.NotifyAbsenceReported(ctx, notificationsSvc.AbsenceReport{
			TenantID: tenantID, StudentIDs: []int64{report.studentID}, Status: report.status, Dates: report.dates,
			FromParent: true, ActorAccountID: report.accountID,
		}); err != nil {
			return nil, err
		}
	}
	s.afterAbsenceCommit(ctx, tenantID, report, rows)
	return rows, nil
}

// afterAbsenceCommit posts the chat pill and wakes the live views once the
// absence has committed.
func (s *Service) afterAbsenceCommit(ctx context.Context, tenantID int64, report absenceReport, rows []*absencerecords.StudentStatusDay) {
	pillBody := SickNoteEventBody(report.status, report.dates)
	pillRefID := firstStatusID(rows)
	tenant.RegisterAfterCommit(ctx, func() {
		s.SelfService.EmitSelfServicePill(tenantID, report.studentID, report.accountID, "sick_note", pillBody, "active.student_status_days", pillRefID)
		s.broadcastStudentUpdated(tenantID, report.studentID)
		// broadcastStudentUpdated is staff-only and emitSelfServicePill wakes
		// just the acting guardian's thread; fan out to EVERY guardian so a
		// co-guardian's open tab drops the stale presence too (#1725 review).
		s.SelfService.WakeChildGuardians(tenantID, report.studentID)
	})
}

// statusDaysFromCarePlan returns Care Plan's status days in the portal's
// response shape.
func statusDaysFromCarePlan(days []careplan.StudentStatusDay) []*absencerecords.StudentStatusDay {
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

func isNewParentReportableAbsence(student *usersModels.Student, status string) bool {
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
func (s *Service) submitAbsenceRequest(ctx context.Context, child *Child, accountID, studentID int64, dates []timezone.Date, note, status string, recipientGuardianProfileIDs []int64) (*SickNoteResult, error) {
	if s.ExcusedRequests == nil {
		return nil, fmt.Errorf("parent: absence request service not configured")
	}
	var req *careplan.ExcusedAbsenceRequest
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
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
		// The note is mandatory only while the school's reason policy asks the
		// family for one (#2267, story 28).
		created, err := s.ExcusedRequests.Submit(txCtx, careplan.ExcusedRequestCreateInput{
			StudentID:         studentID,
			GuardianAccountID: accountID,
			Dates:             carePlanDates(dates),
			Note:              note,
			AbsenceStatus:     status,
			NoteRequired:      s.guardianReasonRequired(ctx, child.TenantID),
		})
		if err != nil {
			return err
		}
		// The recipient choice is written in the SAME transaction as the
		// request: a refused share rolls the request back with it, so a family
		// never ends up with a request nobody they picked can see (#2267).
		if err := s.RequestSharing.ShareRequestInTx(
			txCtx, accountID, studentID, RequestShareExcused, created.ID, recipientGuardianProfileIDs,
		); err != nil {
			return err
		}
		req = created
		return nil
	})
	if txErr != nil {
		return nil, mapExcusedRequestError(txErr, "submit absence request")
	}
	s.Logger.Info("parent submitted absence request",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.TenantID),
		slog.Int64("request_id", req.ID),
		slog.String("status", status),
		slog.Int("days", len(dates)),
	)
	return &SickNoteResult{PendingRequest: req}, nil
}

// applyLiveStatusForParentToday updates the live student flags for a parent
// submission that includes today. A "Krankmeldung" (sick) flips the live sick
// flag on and clears any excused flag, exactly as before. A "Termin/Abwesenheit"
// (excused) sets NO live flag per issue #1735 — it only clears a stale live sick
// flag so the row stays consistent with the now-cleared sick status day, and
// leaves a staff-set excused flag untouched.
func applyLiveStatusForParentToday(student *usersModels.Student, status string, now time.Time) {
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
func studentLiveAbsence(student *usersModels.Student) StudentLiveAbsence {
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
