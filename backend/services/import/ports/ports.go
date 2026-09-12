// Package ports holds the consumer-owned ports of the Data Import workflow
// (#2708). Student, staff and class-list rows are committed through the
// public commands of the owners named here, as do opening balances. A few
// read-only lookups still use retained repositories, bound in
// services/import_composition.go. The import issues no SQL of its own.
// The ports name the owner contracts through their public types, so the
// application and its decision tests depend on this package alone. The
// composition root binds the production capabilities; tests bind the same
// owners over a test database or a recording fake.
package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// People Directory contract: persons, students and guardians.
type (
	Person                   = peopledirectory.Person
	PersonFilter             = peopledirectory.PersonFilter
	CreatePerson             = peopledirectory.CreatePerson
	UpdatePerson             = peopledirectory.UpdatePerson
	Student                  = peopledirectory.Student
	EnrollmentRecord         = peopledirectory.EnrollmentRecord
	EnrollmentStudent        = peopledirectory.EnrollmentStudent
	CreatedEnrollmentStudent = peopledirectory.CreatedEnrollmentStudent
	EnrollmentProfilePatch   = peopledirectory.EnrollmentProfilePatch
	Guardian                 = peopledirectory.Guardian
	GuardianInput            = peopledirectory.GuardianInput
	GuardianPhone            = peopledirectory.GuardianPhone
	GuardianPhoneInput       = peopledirectory.GuardianPhoneInput
	GuardianMatch            = peopledirectory.GuardianMatch
	GuardianWithLink         = peopledirectory.GuardianWithLink
	GuardianLink             = peopledirectory.GuardianLink
	LinkGuardian             = peopledirectory.LinkGuardian
	GuardianLinkUpdate       = peopledirectory.GuardianLinkUpdate
)

// StudentStatusAlumnus marks a graduated child in the directory.
const StudentStatusAlumnus = peopledirectory.StudentStatusAlumnus

var (
	ErrPersonNotFound   = peopledirectory.ErrPersonNotFound
	ErrStudentNotFound  = peopledirectory.ErrStudentNotFound
	ErrGuardianNotFound = peopledirectory.ErrGuardianNotFound
)

// PersonDirectory is the People Directory person slice the import consumes.
type PersonDirectory interface {
	CreatePerson(context.Context, CreatePerson) (Person, error)
	UpdatePerson(context.Context, UpdatePerson) (Person, error)
	FindPerson(context.Context, int64) (Person, error)
	FindPersonByTag(context.Context, string) (Person, error)
	FindPersonByAccount(context.Context, int64) (Person, error)
	SearchPersons(context.Context, PersonFilter) ([]Person, error)
	ListPersonsByID(context.Context, []int64) ([]Person, error)
}

// StudentDirectory is the bounded student owner seam: the same enrollment
// commands the acceptance flow uses, plus the person-to-student lookup.
type StudentDirectory interface {
	ListStudentsByPersonID(context.Context, []int64) ([]Student, error)
	ReadEnrollmentStudent(context.Context, int64, string) (EnrollmentRecord, error)
	CreateEnrollmentStudent(context.Context, EnrollmentStudent) (CreatedEnrollmentStudent, error)
	RenewEnrollmentStudent(context.Context, int64, EnrollmentStudent) error
	ApplyEnrollmentProfile(context.Context, int64, EnrollmentProfilePatch) error
}

// GuardianDirectory is the People Directory guardian slice: profiles, phone
// numbers and the child links.
type GuardianDirectory interface {
	FindGuardianByEmail(context.Context, string) (Guardian, error)
	ListGuardiansByID(context.Context, []int64) ([]Guardian, error)
	ListGuardianPhones(context.Context, int64) ([]GuardianPhone, error)
	ListStudentGuardians(context.Context, int64) ([]GuardianWithLink, error)
	CreateGuardian(context.Context, GuardianInput) (Guardian, error)
	UpdateGuardian(context.Context, int64, GuardianInput) error
	AddGuardianPhone(context.Context, int64, GuardianPhoneInput) (GuardianPhone, error)
	LinkGuardianToStudent(context.Context, LinkGuardian) (GuardianLink, error)
	UpdateGuardianLink(context.Context, int64, GuardianLinkUpdate) error
}

// Care Plan contract: the weekly arrival and pickup schedules.
type (
	ArrivalSchedule       = careplan.ArrivalSchedule
	PickupSchedule        = careplan.PickupSchedule
	StudentScheduleFilter = careplan.StudentScheduleFilter
)

// ScheduleSourceStaff marks a pickup schedule entered by staff (here: the
// importing staff member).
const ScheduleSourceStaff = careplan.ScheduleSourceStaff

// StudentSchedules is the Care Plan slice for the weekly arrival and pickup
// schedules a row may carry.
type StudentSchedules interface {
	ListArrivalSchedules(context.Context, StudentScheduleFilter) ([]ArrivalSchedule, error)
	CreateArrivalSchedule(context.Context, ArrivalSchedule) (ArrivalSchedule, error)
	UpdateArrivalSchedule(context.Context, ArrivalSchedule) error
	ListPickupSchedules(context.Context, StudentScheduleFilter) ([]PickupSchedule, error)
	CreatePickupSchedule(context.Context, PickupSchedule) (PickupSchedule, error)
	UpdatePickupSchedule(context.Context, PickupSchedule) error
}

// Student Presence contract: the retention consent.
type PrivacyConsent = studentpresence.PrivacyConsent

// PrivacyConsents is the Student Presence retention-consent slice.
type PrivacyConsents interface {
	ListPrivacyConsents(context.Context, int64) ([]PrivacyConsent, error)
	RecordPrivacyConsent(context.Context, PrivacyConsent) (PrivacyConsent, error)
}

// School Membership contract: staff, caregiver profiles and class-list
// entries.
type (
	Staff                = schoolmembership.Staff
	StaffFilter          = schoolmembership.StaffFilter
	StaffFields          = schoolmembership.StaffFields
	CreateStaff          = schoolmembership.CreateStaff
	UpdateStaff          = schoolmembership.UpdateStaff
	Teacher              = schoolmembership.Teacher
	TeacherFields        = schoolmembership.TeacherFields
	CreateTeacher        = schoolmembership.CreateTeacher
	UpdateTeacher        = schoolmembership.UpdateTeacher
	ClassListEntry       = schoolmembership.ClassListEntry
	ClassListEntryFilter = schoolmembership.ClassListEntryFilter
	ClassListEntryFields = schoolmembership.ClassListEntryFields
	CreateClassListEntry = schoolmembership.CreateClassListEntry
)

var (
	ErrStaffNotFound   = schoolmembership.ErrStaffNotFound
	ErrTeacherNotFound = schoolmembership.ErrTeacherNotFound
	// ErrMembershipClassListEntryDuplicate is the owner's unique-index
	// sentinel, the race-safe backstop behind the import's own guard.
	ErrMembershipClassListEntryDuplicate = schoolmembership.ErrClassListEntryDuplicate
)

// StaffMembership is the School Membership slice of the staff import.
type StaffMembership interface {
	FindStaff(context.Context, int64) (Staff, error)
	FindStaffByPerson(context.Context, int64) (Staff, error)
	ListStaff(context.Context, StaffFilter) ([]Staff, error)
	CreateStaff(context.Context, CreateStaff) (Staff, error)
	UpdateStaff(context.Context, UpdateStaff) (Staff, error)
	FindTeacherByStaff(context.Context, int64) (Teacher, error)
	CreateTeacher(context.Context, CreateTeacher) (Teacher, error)
	UpdateTeacher(context.Context, UpdateTeacher) (Teacher, error)
}

// ClassListMembership is the School Membership slice of the class-list
// entry import.
type ClassListMembership interface {
	ListClassListEntries(context.Context, ClassListEntryFilter) ([]ClassListEntry, error)
	CreateClassListEntry(context.Context, CreateClassListEntry) (ClassListEntry, error)
}

// Workforce contract: the personnel record.
type (
	StaffMasterData    = workforce.StaffMasterData
	StaffQualification = workforce.StaffQualification
)

var (
	ErrStaffMasterDataNotFound = workforce.ErrStaffMasterDataNotFound
	ErrInvalidStaffRecord      = workforce.ErrInvalidStaffRecord
)

// StaffRecords is the Workforce personnel-record slice.
type StaffRecords interface {
	FindStaffMasterData(context.Context, int64) (StaffMasterData, error)
	CreateStaffMasterData(context.Context, StaffMasterData) (StaffMasterData, error)
	UpdateStaffMasterData(context.Context, StaffMasterData) (StaffMasterData, error)
	ReplaceStaffQualifications(context.Context, int64, []StaffQualification) ([]StaffQualification, error)
}

// AuditCommand is the append-only Audit platform entry point the import
// records its GDPR import log and class-list trail through.
type AuditCommand interface {
	Append(context.Context, any) error
}

// Workforce go-live opening commands and their public values.
type (
	OpeningBalanceBookings    = workforce.OpeningBalanceBookings
	VacationTakeovers         = workforce.VacationTakeovers
	StaffBalanceAdjustment    = workforce.StaffBalanceAdjustment
	StaffVacationOpening      = workforce.StaffVacationOpening
	VacationQuotaSummary      = workforce.VacationQuotaSummary
	SetVacationOpeningRequest = workforce.SetVacationOpeningRequest
)

var (
	ErrOpeningAlreadyExists                = workforce.ErrOpeningAlreadyExists
	ErrVacationOpeningExists               = workforce.ErrVacationOpeningExists
	ErrVacationOpeningAbsencesBeforeCutoff = workforce.ErrVacationOpeningAbsencesBeforeCutoff
	ErrAdjustmentInClosedMonth             = workforce.ErrAdjustmentInClosedMonth
	ErrAdjustmentHasDependentReset         = workforce.ErrAdjustmentHasDependentReset
)
