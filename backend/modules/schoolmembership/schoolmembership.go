// Package schoolmembership is the public School Membership capability. It
// owns users.staff, users.teachers and users.guests: every read or write of
// a staff, teacher or guest row by another owner goes through Query or
// Command instead of a foreign SQL join.
//
// The capability stops at the membership rows themselves. Person names live
// with the People Directory, login accounts and roles with Identity Access;
// compositions that need both resolve them through the respective owner.
package schoolmembership

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"
)

// Employment types a staff row may carry; mirrors the legacy model constants.
const (
	EmploymentTypeFullTime = "full_time"
	EmploymentTypePartTime = "part_time"
	EmploymentTypeMinijob  = "minijob"
)

// DateLayout is the calendar-date wire format of every date field.
const DateLayout = "2006-01-02"

var (
	ErrStaffNotFound           = errors.New("staff member not found")
	ErrTeacherNotFound         = errors.New("teacher not found")
	ErrGuestNotFound           = errors.New("guest not found")
	ErrInvalidMembership       = errors.New("invalid membership")
	ErrStaffPersonConflict     = errors.New("person already has a staff record")
	ErrTeacherStaffConflict    = errors.New("staff member already has a teacher record")
	ErrGuestStaffConflict      = errors.New("staff member already has a guest record")
	ErrPersonnelNumberConflict = errors.New("personnel number is already assigned")
)

// InvalidMembershipError carries the validation reason; it unwraps to
// ErrInvalidMembership so callers can classify it with errors.Is.
type InvalidMembershipError struct{ Reason string }

func (e *InvalidMembershipError) Error() string { return e.Reason }
func (e *InvalidMembershipError) Unwrap() error { return ErrInvalidMembership }

// Staff is one membership row: a person working at the school. A deleted
// staff member carries DeletedAt and is excluded from every query unless the
// filter asks for deleted rows explicitly. RotationAnchorDate is a calendar
// date in DateLayout, empty when unset.
type Staff struct {
	ID                    int64      `json:"id"`
	TenantID              int64      `json:"tenant_id"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	PersonID              int64      `json:"person_id"`
	StaffNotes            string     `json:"staff_notes,omitempty"`
	EmploymentType        *string    `json:"employment_type,omitempty"`
	WorkTimeModelID       *int64     `json:"work_time_model_id,omitempty"`
	PersonnelNumber       *string    `json:"-"`
	RotationAnchorDate    string     `json:"rotation_anchor_date,omitempty"`
	BirthdayDisplayOptOut bool       `json:"birthday_display_opt_out"`
	DeletedAt             *time.Time `json:"-"`
}

func (s Staff) IsDeleted() bool { return s.DeletedAt != nil }

// Teacher is the pedagogical profile of a staff member (Betreuungskraft).
type Teacher struct {
	ID             int64      `json:"id"`
	TenantID       int64      `json:"tenant_id"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	StaffID        int64      `json:"staff_id"`
	Specialization string     `json:"specialization,omitempty"`
	Role           string     `json:"role,omitempty"`
	Qualifications string     `json:"qualifications,omitempty"`
	DeletedAt      *time.Time `json:"-"`
}

func (t Teacher) IsDeleted() bool { return t.DeletedAt != nil }

// Guest is the guest-instructor profile of a staff member. StartDate and
// EndDate are calendar dates in DateLayout, empty when open-ended.
type Guest struct {
	ID                int64     `json:"id"`
	TenantID          int64     `json:"tenant_id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	StaffID           int64     `json:"staff_id"`
	Organization      string    `json:"organization,omitempty"`
	ContactEmail      string    `json:"contact_email,omitempty"`
	ContactPhone      string    `json:"contact_phone,omitempty"`
	ActivityExpertise string    `json:"activity_expertise"`
	StartDate         string    `json:"start_date,omitempty"`
	EndDate           string    `json:"end_date,omitempty"`
	Notes             string    `json:"notes,omitempty"`
}

// StaffFields is the writable part of a staff row shared by create and
// update. RotationAnchorDate is a calendar date in DateLayout or empty.
type StaffFields struct {
	PersonID              int64
	StaffNotes            string
	EmploymentType        *string
	WorkTimeModelID       *int64
	PersonnelNumber       *string
	RotationAnchorDate    string
	BirthdayDisplayOptOut bool
}

type CreateStaff struct{ StaffFields }

type UpdateStaff struct {
	ID int64
	StaffFields
}

type TeacherFields struct {
	StaffID        int64
	Specialization string
	Role           string
	Qualifications string
}

type CreateTeacher struct{ TeacherFields }

type UpdateTeacher struct {
	ID int64
	TeacherFields
}

type GuestFields struct {
	StaffID           int64
	Organization      string
	ContactEmail      string
	ContactPhone      string
	ActivityExpertise string
	StartDate         string
	EndDate           string
	Notes             string
}

type CreateGuest struct{ GuestFields }

type UpdateGuest struct {
	ID int64
	GuestFields
}

// StaffFilter narrows a staff listing. Every field is optional; an empty
// filter lists every live staff member visible in the caller's transaction.
type StaffFilter struct {
	IDs             []int64
	PersonIDs       []int64
	WorkTimeModelID *int64
	// TenantIDs bounds the listing to the given schools. A cross-tenant
	// reader (admin transaction) that knows its schools up front passes them
	// here, so the query stays bounded to those schools instead of scanning
	// the whole platform. Nil means "every school visible in the transaction".
	TenantIDs []int64
	// IncludeDeleted also returns soft-deleted rows; history readers use it
	// to keep resolving offboarded colleagues.
	IncludeDeleted bool
}

// TeacherFilter narrows a teacher listing. The *Contains matches are
// case-insensitive substring matches; Specialization is a case-insensitive
// exact match.
type TeacherFilter struct {
	IDs                    []int64
	StaffIDs               []int64
	Specialization         string
	SpecializationContains string
	RoleContains           string
	HasQualifications      *bool
	IncludeDeleted         bool
}

// GuestFilter narrows a guest listing. ActiveOn is a calendar date in
// DateLayout: only guests whose start/end window covers that day match.
type GuestFilter struct {
	IDs                  []int64
	StaffIDs             []int64
	ActiveOn             string
	OrganizationContains string
	ExpertiseContains    string
	HasOrganization      *bool
}

type Query interface {
	// FindStaff returns one live staff member visible in the caller's
	// transaction.
	FindStaff(context.Context, int64) (Staff, error)
	// FindStaffForMutation locks the row for the caller's transaction.
	FindStaffForMutation(context.Context, int64) (Staff, error)
	FindStaffByPerson(context.Context, int64) (Staff, error)
	// ListStaff returns the staff rows matching the filter, sorted by ID.
	ListStaff(context.Context, StaffFilter) ([]Staff, error)

	FindTeacher(context.Context, int64) (Teacher, error)
	// FindTeacherByStaff returns the live teacher profile of a staff member;
	// a staff member without one yields ErrTeacherNotFound.
	FindTeacherByStaff(context.Context, int64) (Teacher, error)
	ListTeachers(context.Context, TeacherFilter) ([]Teacher, error)

	FindGuest(context.Context, int64) (Guest, error)
	FindGuestByStaff(context.Context, int64) (Guest, error)
	ListGuests(context.Context, GuestFilter) ([]Guest, error)
}

type Command interface {
	CreateStaff(context.Context, CreateStaff) (Staff, error)
	UpdateStaff(context.Context, UpdateStaff) (Staff, error)
	// DeleteStaff soft-deletes the staff row (deleted_at), keeping it.
	DeleteStaff(context.Context, int64) error
	// ClearWorkTimeModel detaches the staff member from their work-time
	// template. Offboarding uses it so the retained row does not block
	// template deletion.
	ClearWorkTimeModel(context.Context, int64) error
	// AppendStaffNotes adds a paragraph to the private staff notes.
	AppendStaffNotes(context.Context, int64, string) (Staff, error)
	SetBirthdayDisplayOptOut(context.Context, int64, bool) error
	// RebaseWorkTimeModelAnchor stamps the template's rotation anchor onto
	// every live staff member assigned to it and returns their IDs.
	RebaseWorkTimeModelAnchor(ctx context.Context, workTimeModelID int64, anchorDate string) ([]int64, error)

	CreateTeacher(context.Context, CreateTeacher) (Teacher, error)
	UpdateTeacher(context.Context, UpdateTeacher) (Teacher, error)
	// DeleteTeacher soft-deletes the teacher profile.
	DeleteTeacher(context.Context, int64) error

	CreateGuest(context.Context, CreateGuest) (Guest, error)
	UpdateGuest(context.Context, UpdateGuest) (Guest, error)
	// DeleteGuest removes the guest profile; guests carry no tombstone.
	DeleteGuest(context.Context, int64) error
}

type Capability interface {
	Query
	Command
}

type engine interface {
	FindStaff(ctx context.Context, id int64, lock string) (Staff, error)
	FindStaffByPerson(context.Context, int64) (Staff, error)
	ListStaff(context.Context, StaffFilter) ([]Staff, error)
	CreateStaff(context.Context, CreateStaff) (Staff, error)
	UpdateStaff(context.Context, UpdateStaff) (Staff, error)
	DeleteStaff(context.Context, int64) error
	ClearWorkTimeModel(context.Context, int64) error
	AppendStaffNotes(context.Context, int64, string) (Staff, error)
	SetBirthdayDisplayOptOut(context.Context, int64, bool) error
	RebaseWorkTimeModelAnchor(context.Context, int64, string) ([]int64, error)

	FindTeacher(context.Context, int64) (Teacher, error)
	FindTeacherByStaff(context.Context, int64) (Teacher, error)
	ListTeachers(context.Context, TeacherFilter) ([]Teacher, error)
	CreateTeacher(context.Context, CreateTeacher) (Teacher, error)
	UpdateTeacher(context.Context, UpdateTeacher) (Teacher, error)
	DeleteTeacher(context.Context, int64) error

	FindGuest(context.Context, int64) (Guest, error)
	FindGuestByStaff(context.Context, int64) (Guest, error)
	ListGuests(context.Context, GuestFilter) ([]Guest, error)
	CreateGuest(context.Context, CreateGuest) (Guest, error)
	UpdateGuest(context.Context, UpdateGuest) (Guest, error)
	DeleteGuest(context.Context, int64) error
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("school membership: engine is required")
	}
	return &Module{engine: engine}
}

// --- staff ---

func (m *Module) FindStaff(ctx context.Context, id int64) (Staff, error) {
	if id <= 0 {
		return Staff{}, invalid("staff ID is required")
	}
	return m.engine.FindStaff(ctx, id, "")
}

func (m *Module) FindStaffForMutation(ctx context.Context, id int64) (Staff, error) {
	if id <= 0 {
		return Staff{}, invalid("staff ID is required")
	}
	return m.engine.FindStaff(ctx, id, "UPDATE")
}

func (m *Module) FindStaffByPerson(ctx context.Context, personID int64) (Staff, error) {
	if personID <= 0 {
		return Staff{}, invalid("person ID is required")
	}
	return m.engine.FindStaffByPerson(ctx, personID)
}

func (m *Module) ListStaff(ctx context.Context, filter StaffFilter) ([]Staff, error) {
	filter.IDs = uniquePositive(filter.IDs)
	filter.PersonIDs = uniquePositive(filter.PersonIDs)
	filter.TenantIDs = uniquePositive(filter.TenantIDs)
	if filter.WorkTimeModelID != nil && *filter.WorkTimeModelID <= 0 {
		return nil, invalid("work time model ID must be positive")
	}
	return m.engine.ListStaff(ctx, filter)
}

func (m *Module) CreateStaff(ctx context.Context, input CreateStaff) (Staff, error) {
	if err := validateStaffFields(&input.StaffFields); err != nil {
		return Staff{}, err
	}
	return m.engine.CreateStaff(ctx, input)
}

func (m *Module) UpdateStaff(ctx context.Context, input UpdateStaff) (Staff, error) {
	if input.ID <= 0 {
		return Staff{}, invalid("staff ID is required")
	}
	if err := validateStaffFields(&input.StaffFields); err != nil {
		return Staff{}, err
	}
	return m.engine.UpdateStaff(ctx, input)
}

func (m *Module) DeleteStaff(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("staff ID is required")
	}
	return m.engine.DeleteStaff(ctx, id)
}

func (m *Module) ClearWorkTimeModel(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("staff ID is required")
	}
	return m.engine.ClearWorkTimeModel(ctx, id)
}

func (m *Module) AppendStaffNotes(ctx context.Context, id int64, notes string) (Staff, error) {
	if id <= 0 {
		return Staff{}, invalid("staff ID is required")
	}
	return m.engine.AppendStaffNotes(ctx, id, notes)
}

func (m *Module) SetBirthdayDisplayOptOut(ctx context.Context, id int64, optOut bool) error {
	if id <= 0 {
		return invalid("staff ID is required")
	}
	return m.engine.SetBirthdayDisplayOptOut(ctx, id, optOut)
}

func (m *Module) RebaseWorkTimeModelAnchor(ctx context.Context, workTimeModelID int64, anchorDate string) ([]int64, error) {
	if workTimeModelID <= 0 {
		return nil, invalid("work time model ID is required")
	}
	if err := validateDate(anchorDate, "rotation anchor date"); err != nil {
		return nil, err
	}
	return m.engine.RebaseWorkTimeModelAnchor(ctx, workTimeModelID, anchorDate)
}

// --- teachers ---

func (m *Module) FindTeacher(ctx context.Context, id int64) (Teacher, error) {
	if id <= 0 {
		return Teacher{}, invalid("teacher ID is required")
	}
	return m.engine.FindTeacher(ctx, id)
}

func (m *Module) FindTeacherByStaff(ctx context.Context, staffID int64) (Teacher, error) {
	if staffID <= 0 {
		return Teacher{}, invalid("staff ID is required")
	}
	return m.engine.FindTeacherByStaff(ctx, staffID)
}

func (m *Module) ListTeachers(ctx context.Context, filter TeacherFilter) ([]Teacher, error) {
	filter.IDs = uniquePositive(filter.IDs)
	filter.StaffIDs = uniquePositive(filter.StaffIDs)
	filter.Specialization = strings.TrimSpace(filter.Specialization)
	filter.SpecializationContains = strings.TrimSpace(filter.SpecializationContains)
	filter.RoleContains = strings.TrimSpace(filter.RoleContains)
	return m.engine.ListTeachers(ctx, filter)
}

func (m *Module) CreateTeacher(ctx context.Context, input CreateTeacher) (Teacher, error) {
	if err := validateTeacherFields(&input.TeacherFields); err != nil {
		return Teacher{}, err
	}
	return m.engine.CreateTeacher(ctx, input)
}

func (m *Module) UpdateTeacher(ctx context.Context, input UpdateTeacher) (Teacher, error) {
	if input.ID <= 0 {
		return Teacher{}, invalid("teacher ID is required")
	}
	if err := validateTeacherFields(&input.TeacherFields); err != nil {
		return Teacher{}, err
	}
	return m.engine.UpdateTeacher(ctx, input)
}

func (m *Module) DeleteTeacher(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("teacher ID is required")
	}
	return m.engine.DeleteTeacher(ctx, id)
}

// --- guests ---

func (m *Module) FindGuest(ctx context.Context, id int64) (Guest, error) {
	if id <= 0 {
		return Guest{}, invalid("guest ID is required")
	}
	return m.engine.FindGuest(ctx, id)
}

func (m *Module) FindGuestByStaff(ctx context.Context, staffID int64) (Guest, error) {
	if staffID <= 0 {
		return Guest{}, invalid("staff ID is required")
	}
	return m.engine.FindGuestByStaff(ctx, staffID)
}

func (m *Module) ListGuests(ctx context.Context, filter GuestFilter) ([]Guest, error) {
	filter.IDs = uniquePositive(filter.IDs)
	filter.StaffIDs = uniquePositive(filter.StaffIDs)
	filter.OrganizationContains = strings.TrimSpace(filter.OrganizationContains)
	filter.ExpertiseContains = strings.TrimSpace(filter.ExpertiseContains)
	if err := validateDate(filter.ActiveOn, "active-on date"); err != nil {
		return nil, err
	}
	return m.engine.ListGuests(ctx, filter)
}

func (m *Module) CreateGuest(ctx context.Context, input CreateGuest) (Guest, error) {
	if err := validateGuestFields(&input.GuestFields); err != nil {
		return Guest{}, err
	}
	return m.engine.CreateGuest(ctx, input)
}

func (m *Module) UpdateGuest(ctx context.Context, input UpdateGuest) (Guest, error) {
	if input.ID <= 0 {
		return Guest{}, invalid("guest ID is required")
	}
	if err := validateGuestFields(&input.GuestFields); err != nil {
		return Guest{}, err
	}
	return m.engine.UpdateGuest(ctx, input)
}

func (m *Module) DeleteGuest(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("guest ID is required")
	}
	return m.engine.DeleteGuest(ctx, id)
}

// --- validation ---

func validateStaffFields(fields *StaffFields) error {
	if fields.PersonID <= 0 {
		return invalid("person ID is required")
	}
	if fields.EmploymentType != nil {
		switch *fields.EmploymentType {
		case EmploymentTypeFullTime, EmploymentTypePartTime, EmploymentTypeMinijob:
		default:
			return invalid("employment_type must be 'full_time', 'part_time', or 'minijob'")
		}
	}
	if fields.WorkTimeModelID != nil && *fields.WorkTimeModelID <= 0 {
		return invalid("work time model ID must be positive")
	}
	if fields.PersonnelNumber != nil {
		trimmed := strings.TrimSpace(*fields.PersonnelNumber)
		if trimmed == "" {
			fields.PersonnelNumber = nil
		} else {
			fields.PersonnelNumber = &trimmed
		}
	}
	return validateDate(fields.RotationAnchorDate, "rotation anchor date")
}

func validateTeacherFields(fields *TeacherFields) error {
	if fields.StaffID <= 0 {
		return invalid("staff ID is required")
	}
	fields.Specialization = strings.TrimSpace(fields.Specialization)
	fields.Role = strings.TrimSpace(fields.Role)
	return nil
}

func validateGuestFields(fields *GuestFields) error {
	if fields.StaffID <= 0 {
		return invalid("staff ID is required")
	}
	fields.ActivityExpertise = strings.TrimSpace(fields.ActivityExpertise)
	if fields.ActivityExpertise == "" {
		return invalid("activity expertise is required")
	}
	fields.Organization = strings.TrimSpace(fields.Organization)
	fields.ContactEmail = strings.TrimSpace(fields.ContactEmail)
	if fields.ContactEmail != "" {
		if _, err := mail.ParseAddress(fields.ContactEmail); err != nil {
			return invalid("invalid contact email format")
		}
	}
	fields.ContactPhone = strings.TrimSpace(fields.ContactPhone)
	if fields.ContactPhone != "" && !validPhone(fields.ContactPhone) {
		return invalid("invalid contact phone format")
	}
	if err := validateDate(fields.StartDate, "start date"); err != nil {
		return err
	}
	if err := validateDate(fields.EndDate, "end date"); err != nil {
		return err
	}
	if fields.StartDate != "" && fields.EndDate != "" && fields.EndDate < fields.StartDate {
		return invalid("end date cannot be before start date")
	}
	return nil
}

// validPhone mirrors the shared phone rule of the legacy guest model: an
// optional leading plus, then digits with spaces, slashes, dashes or
// parentheses, at least six digits in total.
func validPhone(value string) bool {
	digits := 0
	for index, r := range value {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '+' && index == 0:
		case r == ' ' || r == '/' || r == '-' || r == '(' || r == ')':
		default:
			return false
		}
	}
	return digits >= 6 && digits <= 20
}

func validateDate(value, label string) error {
	if value == "" {
		return nil
	}
	if _, err := time.Parse(DateLayout, value); err != nil {
		return invalid(label + " must be a calendar date in YYYY-MM-DD format")
	}
	return nil
}

func uniquePositive(ids []int64) []int64 {
	if ids == nil {
		return nil
	}
	result := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func invalid(reason string) error { return &InvalidMembershipError{Reason: reason} }

// ErrorCode is the stable label recorded per operation outcome.
func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrStaffNotFound), errors.Is(err, ErrTeacherNotFound), errors.Is(err, ErrGuestNotFound):
		return "not_found"
	case errors.Is(err, ErrInvalidMembership):
		return "invalid"
	case errors.Is(err, ErrStaffPersonConflict), errors.Is(err, ErrTeacherStaffConflict), errors.Is(err, ErrGuestStaffConflict):
		return "membership_conflict"
	case errors.Is(err, ErrPersonnelNumberConflict):
		return "personnel_number_conflict"
	default:
		return "internal_error"
	}
}
