package domain

import (
	"errors"
	"time"
)

var (
	ErrStaffNotFound           = errors.New("staff member not found")
	ErrTeacherNotFound         = errors.New("teacher not found")
	ErrGuestNotFound           = errors.New("guest not found")
	ErrStaffPersonConflict     = errors.New("person already has a staff record")
	ErrTeacherStaffConflict    = errors.New("staff member already has a teacher record")
	ErrGuestStaffConflict      = errors.New("staff member already has a guest record")
	ErrPersonnelNumberConflict = errors.New("personnel number is already assigned")
)

type Staff struct {
	ID                    int64
	TenantID              int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
	PersonID              int64
	StaffNotes            string
	EmploymentType        *string
	WorkTimeModelID       *int64
	PersonnelNumber       *string
	RotationAnchorDate    string
	BirthdayDisplayOptOut bool
	DeletedAt             *time.Time
}

func (s Staff) IsDeleted() bool { return s.DeletedAt != nil }

type StaffFields struct {
	PersonID              int64
	StaffNotes            string
	EmploymentType        *string
	WorkTimeModelID       *int64
	PersonnelNumber       *string
	RotationAnchorDate    string
	BirthdayDisplayOptOut bool
}

type StaffFilter struct {
	IDs             []int64
	PersonIDs       []int64
	WorkTimeModelID *int64
	TenantIDs       []int64
	IncludeDeleted  bool
}

type Teacher struct {
	ID             int64
	TenantID       int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StaffID        int64
	Specialization string
	Role           string
	Qualifications string
	DeletedAt      *time.Time
}

func (t Teacher) IsDeleted() bool { return t.DeletedAt != nil }

type TeacherFields struct {
	StaffID        int64
	Specialization string
	Role           string
	Qualifications string
}

type TeacherFilter struct {
	IDs                    []int64
	StaffIDs               []int64
	Specialization         string
	SpecializationContains string
	RoleContains           string
	HasQualifications      *bool
	IncludeDeleted         bool
}

type Guest struct {
	ID                int64
	TenantID          int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	StaffID           int64
	Organization      string
	ContactEmail      string
	ContactPhone      string
	ActivityExpertise string
	StartDate         string
	EndDate           string
	Notes             string
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

type GuestFilter struct {
	IDs                  []int64
	StaffIDs             []int64
	ActiveOn             string
	OrganizationContains string
	ExpertiseContains    string
	HasOrganization      *bool
}

type OperationStats struct {
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
}

func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.StatementDuration += other.StatementDuration
}

// AppendNotes joins a new paragraph onto existing staff notes the way the
// legacy model did: a newline between paragraphs, no leading newline.
func AppendNotes(existing, notes string) string {
	if existing == "" {
		return notes
	}
	return existing + "\n" + notes
}
