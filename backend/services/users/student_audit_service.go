package users

import (
	"context"
	"log/slog"
	"strings"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// StudentAuditService records and reads the per-child change history (issue
// #1455): who changed which student profile field, from what to what, and when.
//
// Scope (stage 1) is the profile fields the OGS office asks about — lifecycle
// status, the three note fields, and the pickup/departure plan. Attendance and
// scheduled status (sick/excused) originate from devices/parents and have their
// own history, so they are intentionally not recorded here.
//
// Values are stored as ready-to-display German strings so the change-history
// view stays a plain "Vorher → Nachher" table with no field-specific frontend
// logic.
type StudentAuditService interface {
	// RecordChanges diffs the before/after student snapshots and appends one
	// audit row per changed tracked field. A no-op when nothing tracked changed.
	RecordChanges(ctx context.Context, before, after *userModels.Student, editedBy int64, editedByName string) error

	// RecordSystemStatusChange records an automated lifecycle transition.
	RecordSystemStatusChange(ctx context.Context, studentID int64, before, after userModels.StudentStatus) error

	// GetChangeHistory returns the student's change history, newest first.
	GetChangeHistory(ctx context.Context, studentID int64) ([]*auditModels.StudentFieldEdit, error)
}

type studentAuditService struct {
	repo   auditModels.StudentFieldEditRepository
	logger *slog.Logger
}

// NewStudentAuditService creates a StudentAuditService.
func NewStudentAuditService(repo auditModels.StudentFieldEditRepository, logger *slog.Logger) StudentAuditService {
	return &studentAuditService{repo: repo, logger: logger}
}

func (s *studentAuditService) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

func (s *studentAuditService) RecordChanges(ctx context.Context, before, after *userModels.Student, editedBy int64, editedByName string) error {
	if before == nil || after == nil {
		return nil
	}

	edits := diffStudentFields(before, after)
	if len(edits) == 0 {
		return nil
	}

	name := strings.TrimSpace(editedByName)
	if name == "" {
		name = "Unbekannt"
	}
	for _, e := range edits {
		e.StudentID = after.ID
		e.EditedBy = editedBy
		e.EditedByName = name
	}

	if err := s.repo.CreateBatch(ctx, edits); err != nil {
		return err
	}

	// GDPR: IDs and field count only, never names or values.
	s.getLogger().Debug("student change history recorded",
		slog.Int64("student_id", after.ID),
		slog.Int64("edited_by", editedBy),
		slog.Int("fields_changed", len(edits)),
	)
	return nil
}

func (s *studentAuditService) RecordSystemStatusChange(
	ctx context.Context,
	studentID int64,
	before userModels.StudentStatus,
	after userModels.StudentStatus,
) error {
	beforeStudent := &userModels.Student{Status: before}
	afterStudent := &userModels.Student{Status: after}
	afterStudent.ID = studentID
	return s.RecordChanges(
		ctx,
		beforeStudent,
		afterStudent,
		auditModels.StudentFieldEditSystemActorID,
		auditModels.StudentFieldEditSystemActorName,
	)
}

func (s *studentAuditService) GetChangeHistory(ctx context.Context, studentID int64) ([]*auditModels.StudentFieldEdit, error) {
	return s.repo.GetByStudentID(ctx, studentID)
}

// diffStudentFields builds an audit edit for each tracked field that changed.
// Each edit carries display-ready German old/new values; the caller fills in
// student ID and editor identity.
func diffStudentFields(before, after *userModels.Student) []*auditModels.StudentFieldEdit {
	var edits []*auditModels.StudentFieldEdit

	add := func(field, oldVal, newVal string) {
		if oldVal == newVal {
			return
		}
		o, n := oldVal, newVal
		edits = append(edits, &auditModels.StudentFieldEdit{
			FieldName: field,
			OldValue:  &o,
			NewValue:  &n,
		})
	}

	add(auditModels.StudentFieldStatus, statusLabel(before.Status), statusLabel(after.Status))
	add(auditModels.StudentFieldSupervisorNotes, derefString(before.SupervisorNotes), derefString(after.SupervisorNotes))
	add(auditModels.StudentFieldExtraInfo, derefString(before.ExtraInfo), derefString(after.ExtraInfo))
	add(auditModels.StudentFieldHealthInfo, derefString(before.HealthInfo), derefString(after.HealthInfo))
	add(auditModels.StudentFieldPickupStatus, derefString(before.PickupStatus), derefString(after.PickupStatus))
	add(auditModels.StudentFieldDepartureDays, departureLabel(before.DepartureDays), departureLabel(after.DepartureDays))

	return edits
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func statusLabel(s userModels.StudentStatus) string {
	switch s {
	case userModels.StudentStatusPending:
		return "Ausstehend"
	case userModels.StudentStatusActive:
		return "Aktiv"
	case userModels.StudentStatusInactive:
		return "Inaktiv"
	case userModels.StudentStatusAlumnus:
		return "Ehemalig"
	default:
		return string(s)
	}
}

var weekdayLabels = map[string]string{
	"mon": "Mo",
	"tue": "Di",
	"wed": "Mi",
	"thu": "Do",
	"fri": "Fr",
}

func departureModeLabel(m userModels.DepartureMode) string {
	switch m {
	case userModels.DepartureBus:
		return "Bus"
	case userModels.DeparturePickup:
		return "Abholung"
	default:
		return "Allein"
	}
}

// departureLabel renders the weekly departure plan as a stable, readable German
// string, e.g. "Mo: Bus, Di: Abholung, Mi: Allein, Do: Allein, Fr: Bus".
func departureLabel(d userModels.DepartureDays) string {
	parts := make([]string, 0, len(userModels.PickupDayOrder))
	for _, day := range userModels.PickupDayOrder {
		parts = append(parts, weekdayLabels[day]+": "+departureModeLabel(d.ModeFor(day)))
	}
	return strings.Join(parts, ", ")
}
