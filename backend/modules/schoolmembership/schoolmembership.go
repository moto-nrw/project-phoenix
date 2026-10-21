// Package schoolmembership is the public School Membership capability. It
// owns users.staff_school_memberships, users.teachers, users.guests,
// users.class_list_entries, education.class_teachers and
// education.group_teacher. Every read or write of those rows by another owner
// goes through Query or Command instead of a foreign SQL join.
//
// A Staff value also carries the employment profile Workforce owns on
// users.staff_employment_profiles (#2753). The module composes it through a
// consumer-owned port and writes it only as part of CreateStaff and
// UpdateStaff; the employment-only commands belong to Workforce.
//
// The capability stops at the membership rows themselves. Person names live
// with the People Directory, login accounts and roles with Identity Access;
// compositions that need both resolve them through the respective owner.
package schoolmembership

import (
	"context"
	"errors"
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
	ErrStaffNotFound     = errors.New("staff member not found")
	ErrTeacherNotFound   = errors.New("teacher not found")
	ErrGuestNotFound     = errors.New("guest not found")
	ErrInvalidMembership = errors.New("invalid membership")
	// ErrStaffPersonMergeNoLongerEligible reports an employment set that no
	// longer satisfies the reviewed staff person repair predicates.
	ErrStaffPersonMergeNoLongerEligible = errors.New("staff person merge no longer eligible")
	ErrStaffPersonConflict              = errors.New("person already has a staff record")
	ErrTeacherStaffConflict             = errors.New("staff member already has a teacher record")
	ErrGuestStaffConflict               = errors.New("staff member already has a guest record")
	ErrPersonnelNumberConflict          = errors.New("personnel number is already assigned")
	// ErrStaffStillReferenced refuses offboarding a staff member other rows
	// still point at. It is user-facing copy and keeps its capital letter.
	ErrStaffStillReferenced = errors.New("personal kann nicht gelöscht werden: Mitarbeiter/in wird noch in anderen Bereichen referenziert")
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
	// PersonalDataErasedAt marks a retired membership the automatic deletion
	// of staff erased (#3963); no Wiederaufnahme continues it.
	PersonalDataErasedAt *time.Time `json:"-"`
	// IsGuest marks an external caregiver without a moto account, recorded
	// as a guest profile (#3823). Time tracking and the duty roster leave
	// these people out; supervision pickers offer them.
	IsGuest           bool   `json:"is_guest"`
	GuestOrganization string `json:"guest_organization,omitempty"`
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
	IDs       []int64
	PersonIDs []int64
	// TenantIDs bounds the listing to the given schools. A cross-tenant
	// reader (admin transaction) that knows its schools up front passes them
	// here, so the query stays bounded to those schools instead of scanning
	// the whole platform. Nil means "every school visible in the transaction".
	TenantIDs []int64
	// IncludeDeleted also returns soft-deleted rows; history readers use it
	// to keep resolving offboarded colleagues.
	IncludeDeleted bool
	// IncludeInactiveGuests keeps the guest marker after the guest's activity
	// window ended. Historical readers use it to classify former employments;
	// ordinary staff reads keep the active-window semantics.
	IncludeInactiveGuests bool
	// MembershipOnly skips the Workforce employment half (#2753): the staff
	// values carry ID, person, tenant and lifecycle only. Callers that read no
	// employment field set it and save a statement.
	MembershipOnly bool
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

// StaffQuery reads the staff rows and their teacher and guest profiles.
type StaffQuery interface {
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

// ClassListQuery reads the Klassenliste entries.
type ClassListQuery interface {
	FindClassListEntry(context.Context, int64) (ClassListEntry, error)
	// FindClassListEntryForMutation locks the row for the caller's transaction.
	FindClassListEntryForMutation(context.Context, int64) (ClassListEntry, error)
	// ListClassListEntries returns the entries matching the filter, ordered
	// by last name, first name (both case-folded) and ID.
	ListClassListEntries(context.Context, ClassListEntryFilter) ([]ClassListEntry, error)
	// ListClassListEntriesInDisplayOrder returns them in class-then-name
	// display order instead.
	ListClassListEntriesInDisplayOrder(context.Context, ClassListEntryFilter) ([]ClassListEntry, error)
	// MatchingStudentIDs names the enrolled students sharing an entry's name
	// and class: the "Zuordnen" hint, never an automatic merge.
	MatchingStudentIDs(context.Context, ClassListEntryFields) ([]int64, error)
}

// TeachingAssignmentQuery reads the class teacher and group teacher rows.
type TeachingAssignmentQuery interface {
	ListClassAssignments(context.Context, ClassAssignmentFilter) ([]ClassAssignment, error)
	ListGroupAssignments(context.Context, GroupAssignmentFilter) ([]GroupAssignment, error)
}

type Query interface {
	StaffQuery
	ClassListQuery
	TeachingAssignmentQuery
}

// StaffCommand writes the staff rows and their teacher and guest profiles.
type StaffCommand interface {
	StaffEmploymentCommands
	CreateStaff(context.Context, CreateStaff) (Staff, error)
	UpdateStaff(context.Context, UpdateStaff) (Staff, error)
	// DeleteStaff soft-deletes the staff row (deleted_at), keeping it.
	DeleteStaff(context.Context, int64) error

	CreateTeacher(context.Context, CreateTeacher) (Teacher, error)
	UpdateTeacher(context.Context, UpdateTeacher) (Teacher, error)
	// DeleteTeacher soft-deletes the teacher profile.
	DeleteTeacher(context.Context, int64) error

	CreateGuest(context.Context, CreateGuest) (Guest, error)
	UpdateGuest(context.Context, UpdateGuest) (Guest, error)
	// DeleteGuest removes the guest profile; guests carry no tombstone.
	DeleteGuest(context.Context, int64) error
}

// ClassListCommand writes the Klassenliste entries.
type ClassListCommand interface {
	CreateClassListEntry(context.Context, CreateClassListEntry) (ClassListEntry, error)
	UpdateClassListEntry(context.Context, UpdateClassListEntry) (ClassListEntry, error)
	// DeleteClassListEntry removes the entry; entries carry no tombstone.
	DeleteClassListEntry(context.Context, int64) error

	// The audited administration commands of the Klassenliste screen. They
	// guard the name against a second entry and against a regular student,
	// and write the change trail in the same transaction.
	AddClassListEntry(context.Context, AddClassListEntry) (ClassListEntry, error)
	ReviseClassListEntry(context.Context, ReviseClassListEntry) (ClassListEntry, error)
	RemoveClassListEntry(context.Context, RemoveClassListEntry) error
	ResolveClassListEntry(context.Context, ResolveClassListEntry) error
}

// TeachingAssignmentCommand writes the class teacher and group teacher rows.
type TeachingAssignmentCommand interface {
	CreateClassAssignment(context.Context, CreateClassAssignment) (ClassAssignment, error)
	UpdateClassAssignment(context.Context, UpdateClassAssignment) (ClassAssignment, error)
	DeleteClassAssignment(context.Context, int64) error
	DeleteClassAssignmentsByStaff(context.Context, int64) (int64, error)

	CreateGroupAssignment(context.Context, CreateGroupAssignment) (GroupAssignment, error)
	UpdateGroupAssignment(context.Context, UpdateGroupAssignment) (GroupAssignment, error)
	DeleteGroupAssignment(context.Context, int64) error
	DeleteGroupAssignmentsByTeacher(context.Context, int64) (int64, error)
	// LockGroupAssignments takes a SHARE ROW EXCLUSIVE table lock on the
	// group assignment rows for the caller's transaction, so a caregiver
	// capability re-check cannot race a concurrent assignment write.
	LockGroupAssignments(context.Context) error
}

type Command interface {
	StaffCommand
	ClassListCommand
	TeachingAssignmentCommand
}

type Capability interface {
	StudentEnrollmentCommands
	Query
	Command
	StaffIdentities
}

// Owner is the whole School Membership owner surface: the Capability and
// the Kinderkontingent usage of the Datenverwaltung. The composition roots
// hand it out; narrower consumers name only the slice they use.
type Owner interface {
	Capability
	ChildQuotaUsages
}

func invalid(reason string) error { return &InvalidMembershipError{Reason: reason} }

// ErrorCode is the stable label recorded per operation outcome.
func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrStaffNotFound), errors.Is(err, ErrTeacherNotFound), errors.Is(err, ErrGuestNotFound),
		errors.Is(err, ErrClassListEntryNotFound), errors.Is(err, ErrClassAssignmentNotFound), errors.Is(err, ErrGroupAssignmentNotFound):
		return "not_found"
	case errors.Is(err, ErrOffboardingConflict):
		return "offboarding_conflict"
	case errors.Is(err, ErrInvalidMembership):
		return "invalid"
	case errors.Is(err, ErrStaffPersonConflict), errors.Is(err, ErrTeacherStaffConflict), errors.Is(err, ErrGuestStaffConflict):
		return "membership_conflict"
	case errors.Is(err, ErrPersonnelNumberConflict):
		return "personnel_number_conflict"
	case errors.Is(err, ErrClassListEntryDuplicate), errors.Is(err, ErrClassListEntryStudentExists),
		errors.Is(err, ErrClassListEntryStudentNotFound), errors.Is(err, ErrClassListEntryAssignMismatch):
		return "class_list_entry_conflict"
	case errors.Is(err, ErrClassAssignmentConflict):
		return "class_assignment_conflict"
	case errors.Is(err, ErrGroupAssignmentConflict):
		return "group_assignment_conflict"
	case errors.Is(err, ErrChildQuotaReached):
		return "child_quota_reached"
	default:
		return "internal_error"
	}
}

// ExternalCaregiver reports whether a staff member is an external caregiver
// (#3823): a guest profile alone is not one; a guest whose person holds no
// account is.
func ExternalCaregiver(guestProfile, personKnown, hasAccount bool) bool {
	return guestProfile && personKnown && !hasAccount
}
