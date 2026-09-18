package timetracking

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
)

var (
	// ErrAbsenceTypeNotFound signals that the requested absence type does not
	// exist for the current tenant.
	ErrAbsenceTypeNotFound = errors.New("diese Abwesenheitsart gibt es nicht")
	// ErrAbsenceTypeInvalid wraps model/input validation failures (maps to 400).
	ErrAbsenceTypeInvalid = errors.New("ungültige Abwesenheitsart")
	// ErrAbsenceTypeNameTaken signals a duplicate name within the tenant.
	ErrAbsenceTypeNameTaken = errors.New("eine Abwesenheitsart mit diesem Namen gibt es bereits")
	// ErrAbsenceTypeNameReserved signals a name that collides with one of the
	// five standard types. Allowing it would put two entries reading "Urlaub"
	// in the dropdown, one of which does not touch the Urlaubskontingent.
	ErrAbsenceTypeNameReserved = errors.New("dieser Name ist bereits eine Standard-Abwesenheitsart")
	// ErrAbsenceTypeInactive signals an attempt to file a new absence under a
	// deactivated art.
	ErrAbsenceTypeInactive = errors.New("diese Abwesenheitsart ist deaktiviert")
	// ErrAbsenceTypeInUse prevents historical absences from changing their
	// display name retroactively through a rename of the referenced art.
	ErrAbsenceTypeInUse = errors.New("eine verwendete Abwesenheitsart kann nicht umbenannt werden")
)

// AbsenceTypeReader supplies custom labels and booking checks from Workforce.
type AbsenceTypeReader interface {
	GetAbsenceType(context.Context, int64) (*activeModels.StaffAbsenceType, error)
	ResolveForAbsence(context.Context, int64) (*activeModels.StaffAbsenceType, error)
	LabelsByID(context.Context) (map[int64]string, error)
	PreviewAllowanceBooking(context.Context, int64, int64, timezone.Date, timezone.Date, bool) ([]*AbsenceTypeAllowanceSummary, error)
	// PreviewAllowanceRebooking checks stored absences against the allowance
	// of the type they would be rebooked to (#3258). With
	// ErrAbsenceTypeAllowanceExceeded the previews are still set.
	PreviewAllowanceRebooking(ctx context.Context, staffID, typeID int64, absenceIDs []int64) ([]*AbsenceTypeAllowanceSummary, error)
}

var (
	ErrAbsenceTypeAllowanceInvalid  = errors.New("ungültiger Anspruch")
	ErrAbsenceTypeAllowanceExceeded = errors.New("kontingent dieser Abwesenheitsart überschritten")
)

type AbsenceTypeAllowanceSummary struct {
	StaffID       int64   `json:"staff_id"`
	AbsenceTypeID int64   `json:"absence_type_id"`
	Year          int     `json:"year"`
	EntitledDays  float64 `json:"entitled_days"`
	TakenDays     float64 `json:"taken_days"`
	ReservedDays  float64 `json:"reserved_days"`
	RemainingDays float64 `json:"remaining_days"`
	// BookingDays is what a previewed booking takes from this year.
	BookingDays float64 `json:"booking_days"`
}

// StampAbsenceTypeLabels enriches custom absences only. Failed label lookups
// preserve existing labels and report through the caller's service logger.
func StampAbsenceTypeLabels(ctx context.Context, svc AbsenceTypeReader, absences []*activeModels.StaffAbsence, logger *slog.Logger) {
	if svc == nil {
		return
	}
	needed := false
	for _, a := range absences {
		if a != nil && a.AbsenceTypeID != nil {
			needed = true
			break
		}
	}
	if !needed {
		return
	}
	labels, err := svc.LabelsByID(ctx)
	if err != nil {
		loggerOrDefault(logger).WarnContext(ctx, "resolving absence type labels failed",
			slog.String("error", err.Error()),
		)
		return
	}
	for _, a := range absences {
		if a == nil || a.AbsenceTypeID == nil {
			continue
		}
		if name, ok := labels[*a.AbsenceTypeID]; ok {
			a.AbsenceTypeLabel = name
		}
	}
}
