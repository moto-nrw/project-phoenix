package workforce

import (
	"context"
	"errors"
	"time"
)

// Canonical staff absence types. A school-defined Abwesenheitsart never
// changes the arithmetic: the row still carries one of these, and every
// calculation keeps reading it.
const (
	AbsenceTypeSick     = "sick"
	AbsenceTypeVacation = "vacation"
	AbsenceTypeTraining = "training"
	AbsenceTypeOther    = "other"
	AbsenceTypeCompTime = "comp_time"
)

// Staff absence lifecycle statuses. reported is an admin-direct entry,
// requested/question are pending decision, approved/declined are terminal,
// canceled is a withdrawal before the decision.
const (
	AbsenceStatusReported  = "reported"
	AbsenceStatusRequested = "requested"
	AbsenceStatusQuestion  = "question"
	AbsenceStatusApproved  = "approved"
	AbsenceStatusDeclined  = "declined"
	AbsenceStatusCanceled  = "canceled"
)

// Overrun policies of a school-defined absence type's own allowance.
const (
	AbsenceTypeOverrunWarn  = "warn"
	AbsenceTypeOverrunBlock = "block"
)

// Date columns a retention cleanup may address on staff absences.
const (
	StaffAbsenceDateStart = "date_start"
	StaffAbsenceDateEnd   = "date_end"
)

var (
	ErrStaffAbsenceNotFound         = errors.New("staff absence not found")
	ErrInvalidStaffAbsence          = errors.New("invalid staff absence input")
	ErrAbsenceTypeNotFound          = errors.New("staff absence type not found")
	ErrAbsenceTypeNameTaken         = errors.New("staff absence type name is already taken")
	ErrAbsenceTypeNameReserved      = errors.New("staff absence type name is a standard type")
	ErrAbsenceTypeInUse             = errors.New("staff absence type is in use")
	ErrAbsenceTypeInactive          = errors.New("staff absence type is inactive")
	ErrAbsenceTypeInvalid           = errors.New("invalid staff absence type")
	ErrAbsenceTypeAllowanceInvalid  = errors.New("invalid staff absence type allowance")
	ErrAbsenceTypeAllowanceExceeded = errors.New("staff absence type allowance exceeded")
)

// InvalidStaffAbsenceError carries the caller-facing validation reason; it
// unwraps to ErrInvalidStaffAbsence so callers classify with errors.Is.
type InvalidStaffAbsenceError struct{ Reason string }

func (e *InvalidStaffAbsenceError) Error() string { return e.Reason }
func (e *InvalidStaffAbsenceError) Unwrap() error { return ErrInvalidStaffAbsence }

func invalidAbsence(reason string) error { return &InvalidStaffAbsenceError{Reason: reason} }

// InvalidAbsenceTypeError carries the caller-facing validation reason of an
// absence type; it unwraps to ErrAbsenceTypeInvalid.
type InvalidAbsenceTypeError struct{ Reason string }

func (e *InvalidAbsenceTypeError) Error() string { return e.Reason }
func (e *InvalidAbsenceTypeError) Unwrap() error { return ErrAbsenceTypeInvalid }

// ConflictError marks a write the database rejected as a duplicate. It keeps
// the driver error reachable through Unwrap, so callers that inspect the
// violated constraint keep working, and reports Kind through errors.Is.
type ConflictError struct {
	Kind  error
	Cause error
}

func (e *ConflictError) Error() string {
	if e.Cause != nil {
		return e.Kind.Error() + ": " + e.Cause.Error()
	}
	return e.Kind.Error()
}

func (e *ConflictError) Is(target error) bool { return target == e.Kind }
func (e *ConflictError) Unwrap() error        { return e.Cause }

// StaffAbsence is one absence record of a staff member. Calendar dates are in
// DateLayout; instants are timestamps.
type StaffAbsence struct {
	ID          int64
	TenantID    int64
	StaffID     int64
	AbsenceType string
	// AbsenceTypeID optionally names the absence with a school-defined
	// Abwesenheitsart; nil means one of the five standard types.
	AbsenceTypeID     *int64
	DateStart         string
	DateEnd           string
	HalfDay           bool
	StartHalfDay      bool
	EndHalfDay        bool
	Note              string
	Status            string
	ApprovedBy        *int64
	ApprovedAt        *time.Time
	CreatedBy         int64
	WorkingDays       *float64
	DecisionNote      string
	RequestedAt       time.Time
	SubstituteStaffID *int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// StaffAbsenceOrderField names a column a listing may be ordered by.
type StaffAbsenceOrderField string

const (
	StaffAbsenceOrderID          StaffAbsenceOrderField = "id"
	StaffAbsenceOrderStaffID     StaffAbsenceOrderField = "staff_id"
	StaffAbsenceOrderDateStart   StaffAbsenceOrderField = "date_start"
	StaffAbsenceOrderDateEnd     StaffAbsenceOrderField = "date_end"
	StaffAbsenceOrderRequestedAt StaffAbsenceOrderField = "requested_at"
)

type StaffAbsenceOrder struct {
	Field      StaffAbsenceOrderField
	Descending bool
}

// StaffAbsenceFilter narrows a listing. Every set field is combined with AND.
// OverlapFrom/OverlapTo select absences intersecting [from, to]; setting both
// to the same day selects the absences covering that day. NonHistoricalFrom
// selects rows still pending (requested/question) or ending on or after the
// day, which is what staff offboarding removes.
type StaffAbsenceFilter struct {
	StaffID           int64
	StaffIDs          []int64
	Statuses          []string
	Types             []string
	OverlapFrom       string
	OverlapTo         string
	DateEndBefore     string
	DateStartBefore   string
	NonHistoricalFrom string
	Order             []StaffAbsenceOrder
	Limit             int
	Offset            int
}

// StaffAbsenceRequestFilter selects absence requests for the Anfragen module.
// Statuses is required. SubjectStaffIDs narrows the rows to those subjects
// when FilterSubjects is set; an empty set then matches nobody.
type StaffAbsenceRequestFilter struct {
	Statuses        []string
	Types           []string
	FilterSubjects  bool
	SubjectStaffIDs []int64
	Limit           int
	// Decided orders the newest decision first; otherwise the oldest request
	// comes first, as the work list expects.
	Decided bool
}

// StaffAbsenceType is a tenant-defined display name for a staff absence. Its
// arithmetic is fixed by BaseType at creation.
type StaffAbsenceType struct {
	ID               int64
	TenantID         int64
	Name             string
	BaseType         string
	IsActive         bool
	AllowanceEnabled bool
	OverrunPolicy    string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// StaffAbsenceTypeFields is the writable part of an absence type.
type StaffAbsenceTypeFields struct {
	Name             string
	BaseType         string
	IsActive         bool
	AllowanceEnabled bool
	OverrunPolicy    string
}

// StaffAbsenceAudit is one status transition of an absence.
type StaffAbsenceAudit struct {
	ID         int64
	TenantID   int64
	AbsenceID  int64
	FromStatus *string
	ToStatus   string
	ActorID    int64
	Note       string
	ChangedAt  time.Time
}

// AbsenceQuery reads staff absences, absence types and the audit trail.
type AbsenceQuery interface {
	FindStaffAbsence(context.Context, int64) (StaffAbsence, error)
	ListStaffAbsences(context.Context, StaffAbsenceFilter) ([]StaffAbsence, error)
	CountStaffAbsences(context.Context, StaffAbsenceFilter) (int, error)
	// ListStaffAbsenceRequests returns absence requests for the Anfragen
	// module; the subject and decider persons are attached by the caller.
	ListStaffAbsenceRequests(context.Context, StaffAbsenceRequestFilter) ([]StaffAbsence, error)
	// StaffAbsenceMapForDate returns staff ID -> canonical absence type of the
	// effective absence on the day. When several overlap, sick > training >
	// vacation > comp_time > other wins.
	StaffAbsenceMapForDate(ctx context.Context, date string) (map[int64]string, error)
	// StaffAbsenceTypeIDMapForDate returns staff ID -> school-defined type of
	// the same winning absence; only staff whose winner carries one appear.
	StaffAbsenceTypeIDMapForDate(ctx context.Context, date string) (map[int64]int64, error)
	// OldestStaffAbsenceDate returns the minimum of column, optionally among
	// rows where column < before; empty when nothing matches.
	OldestStaffAbsenceDate(ctx context.Context, column, before string) (string, error)

	ListStaffAbsenceTypes(context.Context) ([]StaffAbsenceType, error)
	FindStaffAbsenceType(context.Context, int64) (StaffAbsenceType, error)
	// LockStaffAbsenceType returns the type with a transaction-scoped row
	// lock, so a rename or retirement cannot race with a newly filed absence.
	LockStaffAbsenceType(context.Context, int64) (StaffAbsenceType, error)
	// StaffAbsenceTypeInUse reports whether an absence references the type.
	StaffAbsenceTypeInUse(context.Context, int64) (bool, error)
}

// AbsenceCommand writes staff absences, absence types and the audit trail.
type AbsenceCommand interface {
	// LockStaffAbsenceWrites serializes absence lifecycle writes of one staff
	// member inside the ambient transaction. It takes the shared staff
	// balance lock first, because effective absences change the Stundenkonto.
	LockStaffAbsenceWrites(context.Context, int64) error
	CreateStaffAbsence(context.Context, StaffAbsence) (StaffAbsence, error)
	// UpdateStaffAbsence rewrites every column of the absence.
	UpdateStaffAbsence(context.Context, StaffAbsence) (StaffAbsence, error)
	DeleteStaffAbsence(context.Context, int64) error
	// DeleteNonHistoricalStaffAbsences hard-deletes the staff member's
	// pending absences and those ending on or after from.
	DeleteNonHistoricalStaffAbsences(ctx context.Context, staffID int64, from string) (int64, error)
	// DeleteStaffAbsencesOlderThan removes rows whose column is before cutoff.
	DeleteStaffAbsencesOlderThan(ctx context.Context, column, cutoff string) (int64, error)

	CreateStaffAbsenceType(context.Context, StaffAbsenceTypeFields) (StaffAbsenceType, error)
	// UpdateStaffAbsenceType rewrites name, active flag and allowance
	// configuration; the base type is fixed at creation.
	UpdateStaffAbsenceType(context.Context, StaffAbsenceType) (StaffAbsenceType, error)

	RecordStaffAbsenceAudit(context.Context, StaffAbsenceAudit) (StaffAbsenceAudit, error)
}

// AbsenceTypeAllowanceSummary is the yearly account of one staff member for a
// school-defined absence type.
type AbsenceTypeAllowanceSummary struct {
	StaffID       int64
	AbsenceTypeID int64
	Year          int
	EntitledDays  float64
	TakenDays     float64
	ReservedDays  float64
	RemainingDays float64
}

type CreateAbsenceType struct {
	Name             string
	AllowanceEnabled bool
	OverrunPolicy    string
}

// UpdateAbsenceType patches an absence type; nil fields stay as they are, so a
// rename cannot accidentally reactivate a retired type.
type UpdateAbsenceType struct {
	ID               int64
	Name             *string
	IsActive         *bool
	AllowanceEnabled *bool
	OverrunPolicy    *string
}

type SetAbsenceTypeAllowance struct {
	StaffID       int64
	AbsenceTypeID int64
	Year          int
	EntitledDays  float64
	Reason        string
	ChangedBy     int64
}

// AbsenceTypeAdministration is the capability the absence-type administration
// route calls: the school's own Abwesenheitsarten and their per-staff yearly
// allowances. The five standard types are code constants and never appear.
type AbsenceTypeAdministration interface {
	ListAbsenceTypes(context.Context) ([]StaffAbsenceType, error)
	CreateAbsenceType(context.Context, CreateAbsenceType) (StaffAbsenceType, error)
	UpdateAbsenceType(context.Context, UpdateAbsenceType) (StaffAbsenceType, error)
	AllowanceSummary(ctx context.Context, staffID, absenceTypeID int64, year int) (AbsenceTypeAllowanceSummary, error)
	SetAllowance(context.Context, SetAbsenceTypeAllowance) (AbsenceTypeAllowanceSummary, error)
}

type absenceEngine interface {
	AbsenceQuery
	AbsenceCommand
}

// --- staff absences ---

func (m *Module) FindStaffAbsence(ctx context.Context, id int64) (StaffAbsence, error) {
	if id <= 0 {
		return StaffAbsence{}, invalidAbsence("staff absence ID is required")
	}
	return m.engine.FindStaffAbsence(ctx, id)
}

func (m *Module) ListStaffAbsences(ctx context.Context, filter StaffAbsenceFilter) ([]StaffAbsence, error) {
	return m.engine.ListStaffAbsences(ctx, filter)
}

func (m *Module) CountStaffAbsences(ctx context.Context, filter StaffAbsenceFilter) (int, error) {
	return m.engine.CountStaffAbsences(ctx, filter)
}

func (m *Module) ListStaffAbsenceRequests(ctx context.Context, filter StaffAbsenceRequestFilter) ([]StaffAbsence, error) {
	if len(filter.Statuses) == 0 {
		return nil, invalidAbsence("at least one status is required")
	}
	if filter.FilterSubjects && len(filter.SubjectStaffIDs) == 0 {
		return []StaffAbsence{}, nil
	}
	return m.engine.ListStaffAbsenceRequests(ctx, filter)
}

func (m *Module) StaffAbsenceMapForDate(ctx context.Context, date string) (map[int64]string, error) {
	return m.engine.StaffAbsenceMapForDate(ctx, date)
}

func (m *Module) StaffAbsenceTypeIDMapForDate(ctx context.Context, date string) (map[int64]int64, error) {
	return m.engine.StaffAbsenceTypeIDMapForDate(ctx, date)
}

func (m *Module) OldestStaffAbsenceDate(ctx context.Context, column, before string) (string, error) {
	return m.engine.OldestStaffAbsenceDate(ctx, column, before)
}

func (m *Module) LockStaffAbsenceWrites(ctx context.Context, staffID int64) error {
	if staffID <= 0 {
		return invalidAbsence("staff ID is required")
	}
	return m.engine.LockStaffAbsenceWrites(ctx, staffID)
}

func (m *Module) CreateStaffAbsence(ctx context.Context, absence StaffAbsence) (StaffAbsence, error) {
	return m.engine.CreateStaffAbsence(ctx, absence)
}

func (m *Module) UpdateStaffAbsence(ctx context.Context, absence StaffAbsence) (StaffAbsence, error) {
	if absence.ID <= 0 {
		return StaffAbsence{}, invalidAbsence("staff absence ID is required")
	}
	return m.engine.UpdateStaffAbsence(ctx, absence)
}

func (m *Module) DeleteStaffAbsence(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidAbsence("staff absence ID is required")
	}
	return m.engine.DeleteStaffAbsence(ctx, id)
}

func (m *Module) DeleteNonHistoricalStaffAbsences(ctx context.Context, staffID int64, from string) (int64, error) {
	if staffID <= 0 {
		return 0, invalidAbsence("staff ID is required")
	}
	return m.engine.DeleteNonHistoricalStaffAbsences(ctx, staffID, from)
}

func (m *Module) DeleteStaffAbsencesOlderThan(ctx context.Context, column, cutoff string) (int64, error) {
	return m.engine.DeleteStaffAbsencesOlderThan(ctx, column, cutoff)
}

// --- absence types ---

func (m *Module) ListStaffAbsenceTypes(ctx context.Context) ([]StaffAbsenceType, error) {
	return m.engine.ListStaffAbsenceTypes(ctx)
}

func (m *Module) FindStaffAbsenceType(ctx context.Context, id int64) (StaffAbsenceType, error) {
	if id <= 0 {
		return StaffAbsenceType{}, ErrAbsenceTypeNotFound
	}
	return m.engine.FindStaffAbsenceType(ctx, id)
}

func (m *Module) LockStaffAbsenceType(ctx context.Context, id int64) (StaffAbsenceType, error) {
	if id <= 0 {
		return StaffAbsenceType{}, ErrAbsenceTypeNotFound
	}
	return m.engine.LockStaffAbsenceType(ctx, id)
}

func (m *Module) StaffAbsenceTypeInUse(ctx context.Context, id int64) (bool, error) {
	if id <= 0 {
		return false, nil
	}
	return m.engine.StaffAbsenceTypeInUse(ctx, id)
}

func (m *Module) CreateStaffAbsenceType(ctx context.Context, fields StaffAbsenceTypeFields) (StaffAbsenceType, error) {
	return m.engine.CreateStaffAbsenceType(ctx, fields)
}

func (m *Module) UpdateStaffAbsenceType(ctx context.Context, value StaffAbsenceType) (StaffAbsenceType, error) {
	if value.ID <= 0 {
		return StaffAbsenceType{}, ErrAbsenceTypeNotFound
	}
	return m.engine.UpdateStaffAbsenceType(ctx, value)
}

// --- audit ---

func (m *Module) RecordStaffAbsenceAudit(ctx context.Context, audit StaffAbsenceAudit) (StaffAbsenceAudit, error) {
	return m.engine.RecordStaffAbsenceAudit(ctx, audit)
}
