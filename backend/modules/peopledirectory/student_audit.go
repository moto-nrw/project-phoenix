package peopledirectory

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// Tracked field names of the per-child change history (#1455). They are the
// stored discriminators of the trail, so consumers filter and render by them.
const (
	StudentFieldStatus                 = "status"
	StudentFieldSupervisorNotes        = "supervisor_notes"
	StudentFieldExtraInfo              = "extra_info"
	StudentFieldHealthInfo             = "health_info"
	StudentFieldPickupStatus           = "pickup_status"
	StudentFieldPickupSchedule         = "pickup_schedule"
	StudentFieldCareEnd                = "care_end"
	StudentFieldDepartureDays          = "departure_days"
	StudentFieldDepartureCompanionNote = "departure_companion_note"
)

// StudentAuditSystemActorID and …Name identify automated changes. The trail's
// editor column has no foreign key, so zero represents the scheduler without
// attributing its work to a real account.
const (
	StudentAuditSystemActorID   int64 = 0
	StudentAuditSystemActorName       = "System"
)

// StudentAuditSnapshot is the tracked slice of a child's profile: the fields
// the OGS office asks about. Attendance and scheduled status originate from
// devices and parents and have their own history, so they are deliberately
// absent. CareEnd is a calendar date in BirthdayLayout, empty when unset.
type StudentAuditSnapshot struct {
	StudentID              int64
	Status                 string
	CareEnd                string
	SupervisorNotes        *string
	ExtraInfo              *string
	HealthInfo             *string
	PickupStatus           *string
	DepartureCompanionNote *string
	AllowedDepartureModes  departure.AllowedDepartureModes
	DepartureDays          departure.DepartureDays
}

// StudentFieldEdit is one recorded change, with ready-to-display German
// values so the change-history view stays a plain "Vorher → Nachher" table.
type StudentFieldEdit struct {
	ID           int64     `json:"id"`
	StudentID    int64     `json:"student_id"`
	EditedBy     int64     `json:"edited_by"`
	EditedByName string    `json:"edited_by_name"`
	FieldName    string    `json:"field_name"`
	OldValue     *string   `json:"old_value,omitempty"`
	NewValue     *string   `json:"new_value,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// StudentAuditQuery reads the per-child change history.
type StudentAuditQuery interface {
	// ListStudentChangeHistory returns the child's recorded changes, newest
	// first.
	ListStudentChangeHistory(context.Context, int64) ([]StudentFieldEdit, error)
}

// StudentAuditCommand appends to it. Both join the caller's transaction, so a
// recorded change commits or rolls back with the write it describes.
type StudentAuditCommand interface {
	// RecordStudentChanges appends one entry per changed tracked field. A
	// no-op when nothing tracked changed.
	RecordStudentChanges(ctx context.Context, before, after StudentAuditSnapshot, editedBy int64, editedByName string) error
	// RecordStudentPickupPlan appends one entry for a permanent weekly
	// pickup-plan change.
	RecordStudentPickupPlan(ctx context.Context, studentID int64, before, after, result, reason string, editedBy int64, editedByName string) error
}

func (m *Module) ListStudentChangeHistory(ctx context.Context, studentID int64) ([]StudentFieldEdit, error) {
	if studentID <= 0 {
		return nil, invalidStudent("student ID is required")
	}
	return m.engine.ListStudentChangeHistory(ctx, studentID)
}

func (m *Module) RecordStudentChanges(
	ctx context.Context,
	before, after StudentAuditSnapshot,
	editedBy int64,
	editedByName string,
) error {
	if after.StudentID <= 0 {
		return invalidStudent("student ID is required")
	}
	return m.engine.RecordStudentChanges(ctx, before, after, editedBy, editedByName)
}

func (m *Module) RecordStudentPickupPlan(
	ctx context.Context,
	studentID int64,
	before, after, result, reason string,
	editedBy int64,
	editedByName string,
) error {
	if studentID <= 0 {
		return invalidStudent("student ID is required")
	}
	return m.engine.RecordStudentPickupPlan(ctx, studentID, before, after, result, reason, editedBy, editedByName)
}
