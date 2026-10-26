package timetracking

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// OwnAbsenceWrites are the staff member's absence writes that may cascade a
// sick report into Dienstplan and Betreuungsplan (#1843). The root binds the
// shift-plan-sync workflow's entry, which runs Workforce's write and the
// cascade in one tenant transaction (#4825).
type OwnAbsenceWrites interface {
	CreateOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, request workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, error)
	UpdateAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64, request workforce.UpdateAbsenceRequest) (*workforce.StaffAbsenceResponse, error)
	DeleteOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64) error
}

// Resource defines the time-tracking API resource: the staff member's own
// stamps, history, absences and planning views under /api/time-tracking.
type Resource struct {
	WorkSessionService  workforce.OwnWorkSessions
	StaffAbsenceService workforce.OwnAbsences
	AbsenceWrites       OwnAbsenceWrites
	PersonService       workforce.StaffDirectory
	StaffShiftService   workforce.OwnShiftQuery
	// StaffAssignmentService backs GET /assignments — the staff member's own
	// Betreuungsplan blocks (Ort/Aufgabe) for a day (#1844).
	StaffAssignmentService workforce.StaffAssignmentQuery
	// WorkTimeMonthService backs GET /month-summary — the Monatskarte (#1842).
	WorkTimeMonthService workforce.WorkTimeMonths
	// Calendar backs GET /holidays and GET /closing-days — the tenant's public
	// holidays (#1418 3a) and closure periods (#1418 3b) for calendar marking.
	Calendar workforce.PlanningCalendar
	// allowances backs GET /absence-allowances — the staff member's own
	// Kontingente of the school's absence types (#4971).
	allowances         workforce.OwnAbsenceAllowances
	allowanceResponses AllowanceResponses
	// accountStartDate resolves the tenant's time-account start day ("" when
	// unset) for GET /config; nil means the setting is not wired.
	accountStartDate func(context.Context) (string, error)
	identity         IdentityFunc
	withinTenant     TenantTransaction
	shiftResponses   ShiftResponses
}

// Dependencies are the collaborators of the time-tracking resource.
type Dependencies struct {
	WorkSessions  workforce.OwnWorkSessions
	StaffAbsences workforce.OwnAbsences
	AbsenceWrites OwnAbsenceWrites
	Staff         workforce.StaffDirectory
	WorkTimeMonth workforce.WorkTimeMonths
	StaffShifts   workforce.OwnShiftQuery
	Assignments   workforce.StaffAssignmentQuery
	Calendar      workforce.PlanningCalendar
	Allowances    workforce.OwnAbsenceAllowances
	// AllowanceResponses is required with Allowances.
	AllowanceResponses AllowanceResponses
	// AccountStartDate resolves the tenant's time-account start day; nil
	// renders an empty start date.
	AccountStartDate func(context.Context) (string, error)
	Identity         IdentityFunc
	WithinTenant     TenantTransaction
	ShiftResponses   ShiftResponses
}

// NewResource creates a new time-tracking resource.
func NewResource(deps Dependencies) *Resource {
	if deps.StaffShifts != nil && deps.ShiftResponses == nil {
		panic("time tracking resource: shift responses are required with staff shifts")
	}
	if deps.WorkSessions == nil || deps.StaffAbsences == nil || deps.AbsenceWrites == nil || deps.Staff == nil || deps.Allowances == nil || deps.AllowanceResponses == nil || deps.Identity == nil || deps.WithinTenant == nil {
		panic("time tracking resource: work sessions, absences, absence writes, staff directory, allowances with their responses, identity and tenant runtime are required")
	}
	return &Resource{
		WorkSessionService:     deps.WorkSessions,
		StaffAbsenceService:    deps.StaffAbsences,
		AbsenceWrites:          deps.AbsenceWrites,
		PersonService:          deps.Staff,
		StaffShiftService:      deps.StaffShifts,
		StaffAssignmentService: deps.Assignments,
		WorkTimeMonthService:   deps.WorkTimeMonth,
		Calendar:               deps.Calendar,
		allowances:             deps.Allowances,
		allowanceResponses:     deps.AllowanceResponses,
		accountStartDate:       deps.AccountStartDate,
		identity:               deps.Identity,
		withinTenant:           deps.WithinTenant,
		shiftResponses:         deps.ShiftResponses,
	}
}
