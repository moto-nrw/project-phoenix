package timetracking

import (
	"log/slog"

	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
)

// The retained services take their optional collaborators at construction.
// Every option below replaces a former post-construction setter one to one:
// an option left out keeps the behaviour the service had without that
// collaborator, which is the shape the bare unit fixtures rely on.

// WorkSessionOption configures optional work-session collaborators.
type WorkSessionOption func(*workSessionService)

// WithWorkSessionShifts supplies the planned-shift lookups the auto-checkout
// job compares against (#1798). Without it AutoCheckoutDueSessions is a no-op.
func WithWorkSessionShifts(repo WorkSessionShifts) WorkSessionOption {
	return func(s *workSessionService) { s.staffShiftRepo = repo }
}

// WithWorkSessionHolidays supplies the non-working-day resolver that reduces
// the weekly Soll in the history summaries (#1418 3a).
func WithWorkSessionHolidays(reader HolidayDatesReader) WorkSessionOption {
	return func(s *workSessionService) { s.holidayReader = reader }
}

// WithWorkSessionEvents binds the time-account change notification.
func WithWorkSessionEvents(publisher EventPublisher) WorkSessionOption {
	return func(s *workSessionService) { s.broadcaster = publisher }
}

// WithWorkSessionAbsenceTypes supplies the school-defined absence names
// (#2403) the export rows are labelled with.
func WithWorkSessionAbsenceTypes(reader AbsenceTypeReader) WorkSessionOption {
	return func(s *workSessionService) { s.absenceTypes = reader }
}

// WorkTimeMonthOption configures optional month-card collaborators.
type WorkTimeMonthOption func(*workTimeMonthService)

// WithMonthHolidays supplies the public-holiday resolver (#1418 3a). Without
// it no day loses its Soll.
func WithMonthHolidays(reader HolidayDatesReader) WorkTimeMonthOption {
	return func(s *workTimeMonthService) { s.holidayReader = reader }
}

// WithMonthAdjustments supplies the Stundenkonto transaction reader (#1420).
// Without it no adjustment enters the balance.
func WithMonthAdjustments(reader monthAdjustmentReader) WorkTimeMonthOption {
	return func(s *workTimeMonthService) { s.adjustmentRepo = reader }
}

// WithMonthSnapshots supplies the frozen-month reader (#1417). Without it no
// month is ever frozen and the carry chain runs uninterrupted.
func WithMonthSnapshots(reader monthSnapshotReader) WorkTimeMonthOption {
	return func(s *workTimeMonthService) { s.snapshotRepo = reader }
}

// StaffAbsenceOption configures optional absence-service collaborators.
type StaffAbsenceOption func(*staffAbsenceService)

// WithAbsenceLogger supplies the service-scoped logger; bare fixtures fall
// back to slog.Default.
func WithAbsenceLogger(logger *slog.Logger) StaffAbsenceOption {
	return func(s *staffAbsenceService) { s.logger = logger }
}

// WithAbsenceTypes supplies the school-defined absence names (#2403).
func WithAbsenceTypes(reader AbsenceTypeReader) StaffAbsenceOption {
	return func(s *staffAbsenceService) { s.absenceTypes = reader }
}

// WithAbsenceMonthSnapshots supplies the frozen-month reader (#1417) that
// blocks a rebooking inside a closed month (#3258).
func WithAbsenceMonthSnapshots(reader adjustmentFreezeReader) StaffAbsenceOption {
	return func(s *staffAbsenceService) { s.snapshotRepo = reader }
}

// WithAbsenceDeletionAudit supplies the deletion tombstone writer (#1417).
// Without it every delete path fails.
func WithAbsenceDeletionAudit(audit TimeTrackingDeletionAudit) StaffAbsenceOption {
	return func(s *staffAbsenceService) { s.deletionRepo = audit }
}

// WithAbsenceShiftPlanSyncer binds the #1843 sick cascade. Without it the
// service keeps the plans untouched.
func WithAbsenceShiftPlanSyncer(syncer ShiftPlanSyncer) StaffAbsenceOption {
	return func(s *staffAbsenceService) { s.shiftPlanSyncer = syncer }
}

// WithAbsenceEvents binds the time-account change notification.
func WithAbsenceEvents(publisher EventPublisher) StaffAbsenceOption {
	return func(s *staffAbsenceService) { s.broadcaster = publisher }
}

// WithVacationOpenings supplies the vacation takeover repository (#2132).
// Without it summaries carry no takeover and the write paths fail loudly.
func WithVacationOpenings(repo activeModels.StaffVacationOpeningRepository) StaffAbsenceOption {
	return func(s *staffAbsenceService) { s.openingRepo = repo }
}

// WithAbsenceEmail binds the absence email notifications (#1419 4d). Without
// it no email leaves the service.
func WithAbsenceEmail(deps AbsenceEmailDeps) StaffAbsenceOption {
	return func(s *staffAbsenceService) { s.emailDeps = &deps }
}

// StaffBalanceAdjustmentOption configures optional ledger collaborators.
type StaffBalanceAdjustmentOption func(*staffBalanceAdjustmentService)

// WithAdjustmentSnapshots supplies the frozen-month reader (#1417). Without
// it no month counts as frozen.
func WithAdjustmentSnapshots(reader adjustmentFreezeReader) StaffBalanceAdjustmentOption {
	return func(s *staffBalanceAdjustmentService) { s.snapshotRepo = reader }
}

// WithAdjustmentDeletionAudit supplies the deletion tombstone writer (#1417).
// Without it DeleteAdjustment fails: deletes without a trace are exactly the
// hole this closes.
func WithAdjustmentDeletionAudit(audit TimeTrackingDeletionAudit) StaffBalanceAdjustmentOption {
	return func(s *staffBalanceAdjustmentService) { s.deletionRepo = audit }
}

// WithAdjustmentEvents binds the time-account change notification.
func WithAdjustmentEvents(publisher EventPublisher) StaffBalanceAdjustmentOption {
	return func(s *staffBalanceAdjustmentService) { s.broadcaster = publisher }
}

// StaffMonthCloseOption configures optional month-close collaborators.
type StaffMonthCloseOption func(*staffMonthCloseService)

// WithMonthCloseEvents binds the time-account change notification.
func WithMonthCloseEvents(publisher EventPublisher) StaffMonthCloseOption {
	return func(s *staffMonthCloseService) { s.broadcaster = publisher }
}

// StaffOverviewOption configures optional overview collaborators.
type StaffOverviewOption func(*staffOverviewService)

// WithOverviewHolidays supplies the non-working-day resolver.
func WithOverviewHolidays(reader HolidayDatesReader) StaffOverviewOption {
	return func(s *staffOverviewService) { s.holidayReader = reader }
}

// WithOverviewVacationOpenings supplies the vacation takeover reader (#2132).
// Without it the overview ignores takeovers.
func WithOverviewVacationOpenings(repo activeModels.StaffVacationOpeningRepository) StaffOverviewOption {
	return func(s *staffOverviewService) { s.openingRepo = repo }
}
