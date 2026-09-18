package domain

import (
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// Tracked field names of the per-child change history (#1455). They are the
// stored discriminators of audit.student_field_edits, so they are part of the
// recorded data, not a display concern.
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

// StudentFieldEditSystemActorID and …Name identify automated changes.
// edited_by has no foreign key, so zero represents the scheduler without
// attributing its work to a real account.
const (
	StudentFieldEditSystemActorID   int64 = 0
	StudentFieldEditSystemActorName       = "System"
)

// StudentAuditSnapshot is the tracked slice of a child's profile: the fields
// the OGS office asks about. Attendance and scheduled status originate from
// devices and parents and have their own history, so they are deliberately
// absent.
type StudentAuditSnapshot struct {
	StudentID int64
	Status    string
	// CareEnd is the last care day as a calendar date (YYYY-MM-DD), empty
	// when the child has no recorded end of care.
	CareEnd                string
	SupervisorNotes        *string
	ExtraInfo              *string
	HealthInfo             *string
	PickupStatus           *string
	DepartureCompanionNote *string
	AllowedDepartureModes  departure.AllowedDepartureModes
	DepartureDays          departure.DepartureDays
}

// StudentFieldChange is one recorded change. Values are ready-to-display
// German strings so the change-history view stays a plain "Vorher → Nachher"
// table with no field-specific frontend logic.
type StudentFieldChange struct {
	FieldName string
	OldValue  string
	NewValue  string
}

// StudentFieldEdit is one stored change-history row.
type StudentFieldEdit struct {
	ID           int64
	StudentID    int64
	EditedBy     int64
	EditedByName string
	FieldName    string
	OldValue     *string
	NewValue     *string
	CreatedAt    time.Time
}

// DiffStudentFields reports one change per tracked field that moved. The
// caller fills in student ID and editor identity.
func DiffStudentFields(before, after StudentAuditSnapshot) []StudentFieldChange {
	var changes []StudentFieldChange
	add := func(field, oldValue, newValue string) {
		if oldValue == newValue {
			return
		}
		changes = append(changes, StudentFieldChange{FieldName: field, OldValue: oldValue, NewValue: newValue})
	}

	add(StudentFieldStatus, studentStatusLabel(before.Status), studentStatusLabel(after.Status))
	add(StudentFieldSupervisorNotes, derefString(before.SupervisorNotes), derefString(after.SupervisorNotes))
	add(StudentFieldExtraInfo, derefString(before.ExtraInfo), derefString(after.ExtraInfo))
	add(StudentFieldHealthInfo, derefString(before.HealthInfo), derefString(after.HealthInfo))
	add(StudentFieldPickupStatus, derefString(before.PickupStatus), derefString(after.PickupStatus))
	add(StudentFieldCareEnd, careEndLabel(before.CareEnd), careEndLabel(after.CareEnd))
	add(StudentFieldDepartureDays, departurePlanLabel(before), departurePlanLabel(after))
	add(
		StudentFieldDepartureCompanionNote,
		derefString(before.DepartureCompanionNote),
		derefString(after.DepartureCompanionNote),
	)
	return changes
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func studentStatusLabel(status string) string {
	switch status {
	case "pending":
		return "Ausstehend"
	case "active":
		return "Aktiv"
	case "inactive":
		return "Inaktiv"
	default:
		return status
	}
}

// careEndLabel renders the last care day as a German date, or "" when the
// child has no end of care recorded. Empty on both sides is a no-op for the
// diff, so untouched children never grow a history row (#2487).
func careEndLabel(careEnd string) string {
	if careEnd == "" {
		return ""
	}
	day, err := time.Parse("2006-01-02", careEnd)
	if err != nil {
		return careEnd
	}
	return day.Format("02.01.2006")
}

var auditWeekdayLabels = map[string]string{
	"mon": "Mo",
	"tue": "Di",
	"wed": "Mi",
	"thu": "Do",
	"fri": "Fr",
}

// auditDepartureModeLabel is the change history's own short label set. It is
// deliberately NOT departure.DepartureMode.GermanLabel(): these strings are
// stored in audit.student_field_edits and rendered verbatim, so the recorded
// history of every existing tenant would change with them.
func auditDepartureModeLabel(mode departure.DepartureMode) string {
	switch mode {
	case departure.DepartureBus:
		return "Bus"
	case departure.DeparturePickup:
		return "Abholung"
	case departure.DepartureAccompanied:
		return "Mit anderem Kind"
	default:
		return "Allein"
	}
}

// departurePlanLabel renders the weekly departure plan as a stable, readable
// German string, preserving every non-exclusive allowed mode.
func departurePlanLabel(snapshot StudentAuditSnapshot) string {
	modesByDay := snapshot.AllowedDepartureModes.Normalize()
	if !modesByDay.HasAny() {
		modesByDay = departure.AllowedDepartureModesFromDeparture(snapshot.DepartureDays)
	}

	parts := make([]string, 0, len(departure.PickupDayOrder))
	for _, day := range departure.PickupDayOrder {
		modes := modesByDay[day]
		if len(modes) == 0 {
			modes = []departure.DepartureMode{departure.DepartureAlone}
		}
		labels := make([]string, 0, len(modes))
		for _, mode := range modes {
			labels = append(labels, auditDepartureModeLabel(mode))
		}
		parts = append(parts, auditWeekdayLabels[day]+": "+strings.Join(labels, " / "))
	}
	return strings.Join(parts, ", ")
}
