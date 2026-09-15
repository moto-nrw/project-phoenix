package staff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// JSONID accepts a JSON number or a decimal string. Person ids are bigints,
// and the Next.js proxy in front of this endpoint parses and re-serializes
// the body, which rounds a JSON number past 2^53 into a valid id for a
// DIFFERENT person (#2222). A string is parsed with strconv, so every digit
// survives, and anything that is not a plain integer is rejected instead of
// being coerced to zero.
type JSONID int64

// Int64 returns the id as the int64 the rest of the code works with.
func (id JSONID) Int64() int64 { return int64(id) }

func (id *JSONID) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, `"`) {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return errors.New("id must not be empty")
		}
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid id %q: %w", raw, err)
		}
		*id = JSONID(parsed)
		return nil
	}
	var parsed int64
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	*id = JSONID(parsed)
	return nil
}

// MarshalJSON emits a JSON number, so a struct that is decoded and
// re-encoded keeps the shape every existing consumer expects.
func (id JSONID) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(int64(id), 10)), nil
}

// PersonResponse represents a simplified person response.
type PersonResponse struct {
	ID        int64     `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email,omitempty"`
	Avatar    string    `json:"avatar,omitempty"`
	TagID     string    `json:"tag_id,omitempty"`
	AccountID *int64    `json:"account_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StaffResponse represents a staff response.
type StaffResponse struct {
	ID int64 `json:"id"`
	// PersonID goes out as a decimal STRING. It is a bigint, and the staff
	// screens send it back to identify the person they edit — as a JSON
	// number it would be rounded past 2^53 by the JSON.parse in the Next.js
	// proxy, before any mapper could preserve it, and the rounded value is a
	// valid id for a different person (#2222). The request side takes either
	// form (JSONID), so no client breaks on this.
	PersonID        int64           `json:"person_id,string"`
	StaffNotes      string          `json:"staff_notes,omitempty"`
	Person          *PersonResponse `json:"person,omitempty"`
	IsTeacher       bool            `json:"is_teacher"`
	WasPresentToday bool            `json:"was_present_today"`
	WorkStatus      string          `json:"work_status,omitempty"`
	AbsenceType     string          `json:"absence_type,omitempty"`
	// AbsenceTypeLabel carries the school's own Abwesenheitsart wording for
	// today's absence (#2403). Empty for the five standard types — the client
	// keeps deriving those labels from AbsenceType itself.
	AbsenceTypeLabel string    `json:"absence_type_label,omitempty"`
	AccountRole      string    `json:"account_role,omitempty"`
	EmploymentType   *string   `json:"employment_type,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TeacherResponse represents a teacher response (extends staff).
type TeacherResponse struct {
	StaffResponse
	TeacherID      int64  `json:"teacher_id"`
	Specialization string `json:"specialization,omitempty"`
	Role           string `json:"role,omitempty"`
	Qualifications string `json:"qualifications,omitempty"`
}

// GroupResponse represents a simplified group response.
type GroupResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// StaffWithRoleResponse represents a staff member with role information.
type StaffWithRoleResponse struct {
	ID                int64     `json:"id"`
	PersonID          int64     `json:"person_id"`
	TeacherID         int64     `json:"teacher_id,omitempty"`
	FirstName         string    `json:"first_name"`
	LastName          string    `json:"last_name"`
	FullName          string    `json:"full_name"`
	AccountID         int64     `json:"account_id"`
	Email             string    `json:"email"`
	IsActiveCaregiver bool      `json:"is_active_caregiver"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// StaffRequest represents a staff creation/update request.
type StaffRequest struct {
	PersonID   JSONID `json:"person_id"`
	StaffNotes string `json:"staff_notes,omitempty"`
	// Teacher-specific fields for creating a teacher
	IsTeacher      bool   `json:"is_teacher,omitempty"`
	Specialization string `json:"specialization,omitempty"`
	Role           string `json:"role,omitempty"`
	Qualifications string `json:"qualifications,omitempty"`
}

// Bind validates the staff request.
func (req *StaffRequest) Bind(*http.Request) error {
	if req.PersonID <= 0 {
		return errors.New("person ID is required")
	}
	req.Specialization = strings.TrimSpace(req.Specialization)
	req.Role = strings.TrimSpace(req.Role)
	req.Qualifications = strings.TrimSpace(req.Qualifications)
	return nil
}

// SchoolClassesResponse is the wire shape of a staff member's school class
// assignments (#1772).
type SchoolClassesResponse struct {
	StaffID       int64    `json:"staff_id"`
	SchoolClasses []string `json:"school_classes"`
}

// SchoolClassesRequest is the PUT body replacing the assignments.
type SchoolClassesRequest struct {
	SchoolClasses []string `json:"school_classes"`
}

// Bind implements render.Binder.
func (req *SchoolClassesRequest) Bind(*http.Request) error {
	if req.SchoolClasses == nil {
		return errors.New("school_classes is required")
	}
	return nil
}

// PINStatusResponse represents the PIN status response.
type PINStatusResponse struct {
	HasPIN      bool       `json:"has_pin"`
	LastChanged *time.Time `json:"last_changed,omitempty"`
}

// PINUpdateRequest represents a PIN update request.
type PINUpdateRequest struct {
	CurrentPIN *string `json:"current_pin,omitempty"` // null for first-time setup
	NewPIN     string  `json:"new_pin"`
}

// Bind validates the PIN update request.
func (req *PINUpdateRequest) Bind(*http.Request) error {
	if req.NewPIN == "" {
		return errors.New("new PIN is required")
	}
	if len(req.NewPIN) != 4 {
		return errors.New("PIN must be exactly 4 digits")
	}
	for _, char := range req.NewPIN {
		if char < '0' || char > '9' {
			return errors.New("PIN must contain only digits")
		}
	}
	return nil
}

// staffFieldAccess splits the staff directory into the tiers a caller can be
// entitled to beyond the minimal colleague view (#2906).
type staffFieldAccess struct {
	// notes covers the private staff notes an OGS-Leitung keeps on a
	// colleague — the most sensitive part of the record, so it stays with the
	// permission that writes them.
	notes bool
	// qualifications covers the free-text qualifications on the teacher
	// record; maintaining the personnel file reads them too.
	qualifications bool
	// personnel covers employment type, today's absence reason including the
	// school's own wording, and the NFC tag.
	personnel bool
}

func (rs *Resource) fieldAccess(ctx context.Context) staffFieldAccess {
	return fieldAccessFor(rs.runtime.HasPermission, rs.runtime.Permissions(ctx))
}

// fieldAccessFor is the tier mapping on a plain permission list. The matcher
// is wildcard aware, so admin:* matches every tier.
func fieldAccessFor(matches func(string, []string) bool, granted []string) staffFieldAccess {
	has := func(required string) bool { return matches(required, granted) }
	return staffFieldAccess{
		notes:          has(permissions.StaffManage),
		qualifications: has(permissions.StaffManage) || has(permissions.StaffStammdaten),
		personnel:      has(permissions.StaffStammdaten) || has(permissions.TimeTrackingManage),
	}
}

// enrichment carries the per-staff decoration the list and detail views add
// on top of the membership row.
type enrichment struct {
	present          bool
	workStatus       string
	absenceType      string
	absenceTypeLabel string
	accountRole      string
	email            string
	avatar           string
}

func newPersonResponse(person *Person, email, avatar string) *PersonResponse {
	if person == nil {
		return nil
	}
	return &PersonResponse{
		ID:        person.ID,
		FirstName: person.FirstName,
		LastName:  person.LastName,
		Email:     email,
		Avatar:    avatar,
		TagID:     person.TagID,
		AccountID: person.AccountID,
		CreatedAt: person.CreatedAt,
		UpdatedAt: person.UpdatedAt,
	}
}

// buildStaffResponse maps one membership row to the wire shape and strips
// whatever the caller's tiers do not cover. Redaction lives here, in the
// single constructor every staff response goes through, so a new endpoint
// cannot forget it.
func buildStaffResponse(access staffFieldAccess, staff schoolmembership.Staff, person *Person, isTeacher bool, data enrichment) StaffResponse {
	response := StaffResponse{
		ID:               staff.ID,
		PersonID:         staff.PersonID,
		StaffNotes:       staff.StaffNotes,
		IsTeacher:        isTeacher,
		WasPresentToday:  data.present,
		WorkStatus:       data.workStatus,
		AbsenceType:      data.absenceType,
		AbsenceTypeLabel: data.absenceTypeLabel,
		AccountRole:      data.accountRole,
		EmploymentType:   staff.EmploymentType,
		CreatedAt:        staff.CreatedAt,
		UpdatedAt:        staff.UpdatedAt,
	}
	if person != nil {
		response.Person = newPersonResponse(person, data.email, data.avatar)
	}
	redactStaffFields(&response, access)
	return response
}

// redactStaffFields leaves the minimal colleague view (#2906). Keep the field
// lists in sync with StaffResponse — a new field is visible to every
// colleague until it is listed here.
func redactStaffFields(response *StaffResponse, access staffFieldAccess) {
	if !access.notes {
		response.StaffNotes = ""
	}
	if !access.personnel {
		response.AbsenceType = ""
		response.AbsenceTypeLabel = ""
		response.EmploymentType = nil
		if response.Person != nil {
			response.Person.TagID = ""
		}
	}
}

func buildTeacherResponse(access staffFieldAccess, staff schoolmembership.Staff, person *Person, teacher schoolmembership.Teacher, data enrichment) TeacherResponse {
	response := TeacherResponse{
		StaffResponse:  buildStaffResponse(access, staff, person, true, data),
		TeacherID:      teacher.ID,
		Specialization: strings.TrimSpace(teacher.Specialization),
		Role:           teacher.Role,
		Qualifications: teacher.Qualifications,
	}
	// Free-text qualifications are personnel data; Specialization and Role
	// are the pedagogical labels the group and substitution screens display.
	if !access.qualifications {
		response.Qualifications = ""
	}
	return response
}

// buildResponse returns the teacher shape when the staff member carries a
// teacher profile and the plain staff shape otherwise.
func buildResponse(access staffFieldAccess, staff schoolmembership.Staff, person *Person, teacher *schoolmembership.Teacher, data enrichment) any {
	if teacher != nil {
		return buildTeacherResponse(access, staff, person, *teacher, data)
	}
	return buildStaffResponse(access, staff, person, false, data)
}
