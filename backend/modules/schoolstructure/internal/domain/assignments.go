package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The teaching assignments and handovers the group service and the
// substitution module read and write. School Membership owns
// education.group_teacher and education.class_teachers, Workforce owns
// education.group_substitution; these are School Structure's view of their
// rows, which the composition root binds over the owners' capabilities.

// GroupTeacher assigns a teacher to a group (education.group_teacher).
type GroupTeacher struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	GroupID   int64     `json:"group_id"`
	TeacherID int64     `json:"teacher_id"`
}

// Validate requires the group and the teacher.
func (gt *GroupTeacher) Validate() error {
	if gt.GroupID <= 0 {
		return errors.New("group ID is required")
	}
	if gt.TeacherID <= 0 {
		return errors.New("teacher ID is required")
	}
	return nil
}

// ClassTeacher assigns a staff member to a school class (#1772). The class is
// the free-text users.students.school_class value, matched case-insensitively
// on LOWER(BTRIM(...)) — there is deliberately no class entity: assignments
// are redone every school year anyway, and the grade-transition tooling
// rewrites the student strings. SchoolClass stores the display form as
// entered; comparisons must go through schoolclass.Normalize.
type ClassTeacher struct {
	ID          int64     `json:"id"`
	TenantID    int64     `json:"tenant_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	StaffID     int64     `json:"staff_id"`
	SchoolClass string    `json:"school_class"`
}

// Validate requires the staff member and a non-blank class.
func (ct *ClassTeacher) Validate() error {
	if ct.StaffID <= 0 {
		return errors.New("staff ID is required")
	}
	if strings.TrimSpace(ct.SchoolClass) == "" {
		return errors.New("school class is required")
	}
	return nil
}

// The target types of education.group_substitution.
const (
	GroupSubstitutionTypeGroupHandover = "group_handover"
	GroupSubstitutionTypeLegacy        = "legacy_personnel_substitution"
)

// GroupSubstitution is a temporary substitution of a staff member for
// another in a group.
type GroupSubstitution struct {
	ID                int64         `json:"id"`
	TenantID          int64         `json:"tenant_id"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
	TargetType        string        `json:"-"`
	GroupID           int64         `json:"group_id"`
	RegularStaffID    *int64        `json:"regular_staff_id,omitempty"`
	SubstituteStaffID int64         `json:"substitute_staff_id"`
	StartDate         calendar.Date `json:"start_date"`
	EndDate           calendar.Date `json:"end_date"`
	Reason            string        `json:"reason,omitempty"`

	// Relations the reads with relations attach.
	Group           *Group             `json:"group,omitempty"`
	RegularStaff    *SubstitutionStaff `json:"regular_staff,omitempty"`
	SubstituteStaff *SubstitutionStaff `json:"substitute_staff,omitempty"`
}

// SubstitutionStaff is a staff member a substitution names, as School
// Membership resolves it. Person stays nil until the People Directory
// resolved the name.
type SubstitutionStaff struct {
	ID       int64               `json:"id"`
	PersonID int64               `json:"person_id"`
	Person   *SubstitutionPerson `json:"person,omitempty"`
}

// SubstitutionPerson is the name of a substitution's staff member.
type SubstitutionPerson struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// FullName returns "Vorname Nachname".
func (p *SubstitutionPerson) FullName() string {
	return p.FirstName + " " + p.LastName
}
