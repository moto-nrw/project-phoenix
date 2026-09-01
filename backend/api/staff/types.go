package staff

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// PersonResponse represents a simplified person response
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

// StaffResponse represents a staff response
type StaffResponse struct {
	ID int64 `json:"id"`
	// PersonID goes out as a decimal STRING. It is a bigint, and the staff
	// screens send it back to identify the person they edit — as a JSON number
	// it would be rounded past 2^53 by the JSON.parse in the Next.js proxy,
	// before any mapper could preserve it, and the rounded value is a valid id
	// for a different person (#2222). The request side takes either form
	// (common.JSONID), so no client breaks on this.
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

// TeacherResponse represents a teacher response (extends staff)
type TeacherResponse struct {
	StaffResponse
	TeacherID      int64  `json:"teacher_id"`
	Specialization string `json:"specialization,omitempty"`
	Role           string `json:"role,omitempty"`
	Qualifications string `json:"qualifications,omitempty"`
}

// GroupResponse represents a simplified group response
type GroupResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// StaffRequest represents a staff creation/update request
type StaffRequest struct {
	// PersonID takes a JSON number or a decimal string. The staff screens send
	// the string: person ids are bigints, and the Next.js proxy in front of this
	// endpoint parses and re-serializes the body, which rounds a JSON number past
	// 2^53 into a valid id for a different person (#2222).
	PersonID   common.JSONID `json:"person_id"`
	StaffNotes string        `json:"staff_notes,omitempty"`
	// Teacher-specific fields for creating a teacher
	IsTeacher      bool   `json:"is_teacher,omitempty"`
	Specialization string `json:"specialization,omitempty"`
	Role           string `json:"role,omitempty"`
	Qualifications string `json:"qualifications,omitempty"`
}

// PINStatusResponse represents the PIN status response
type PINStatusResponse struct {
	HasPIN      bool       `json:"has_pin"`
	LastChanged *time.Time `json:"last_changed,omitempty"`
}

// PINUpdateRequest represents a PIN update request
type PINUpdateRequest struct {
	CurrentPIN *string `json:"current_pin,omitempty"` // null for first-time setup
	NewPIN     string  `json:"new_pin"`
}

// Bind validates the staff request
func (req *StaffRequest) Bind(_ *http.Request) error {
	if req.PersonID <= 0 {
		return errors.New("person ID is required")
	}

	req.Specialization = strings.TrimSpace(req.Specialization)
	req.Role = strings.TrimSpace(req.Role)
	req.Qualifications = strings.TrimSpace(req.Qualifications)

	return nil
}

// Bind validates the PIN update request
func (req *PINUpdateRequest) Bind(_ *http.Request) error {
	if req.NewPIN == "" {
		return errors.New("new PIN is required")
	}

	// Validate PIN format (4 digits)
	if len(req.NewPIN) != 4 {
		return errors.New("PIN must be exactly 4 digits")
	}

	// Check if PIN contains only digits
	for _, char := range req.NewPIN {
		if char < '0' || char > '9' {
			return errors.New("PIN must contain only digits")
		}
	}

	return nil
}

// newPersonResponse creates a simplified person response
func newPersonResponse(person *users.Person, email string, avatar string) *PersonResponse {
	if person == nil {
		return nil
	}

	response := &PersonResponse{
		ID:        person.ID,
		FirstName: person.FirstName,
		LastName:  person.LastName,
		Email:     email,
		Avatar:    avatar,
		AccountID: person.AccountID,
		CreatedAt: person.CreatedAt,
		UpdatedAt: person.UpdatedAt,
	}

	if person.TagID != nil {
		response.TagID = *person.TagID
	}

	return response
}

// newStaffResponse creates a staff response.
//
// The context decides how much of the record goes on the wire: everyone who
// may see the directory at all (users:read) gets the minimal colleague view —
// name, avatar, account role, teacher flag, work e-mail and today's presence
// state. The personnel-file fields (staff notes, employment type, absence
// reason, NFC tag, free-text qualifications) are added only for the personnel
// administrators (#2906). Redaction lives here, in the single constructor
// every staff response goes through, so a new endpoint cannot forget it.
func newStaffResponse(ctx context.Context, staff *users.Staff, isTeacher bool, wasPresentToday bool, workStatus string, absenceType string, accountRole string, email string, avatar string) StaffResponse {
	return buildStaffResponse(canSeeStaffPersonnelFields(ctx), staff, isTeacher, wasPresentToday, workStatus, absenceType, accountRole, email, avatar)
}

// buildStaffResponse is newStaffResponse with the personnel decision already
// made, so the mapping can be tested without minting a JWT context.
func buildStaffResponse(showPersonnel bool, staff *users.Staff, isTeacher bool, wasPresentToday bool, workStatus string, absenceType string, accountRole string, email string, avatar string) StaffResponse {
	response := StaffResponse{
		ID:              staff.ID,
		PersonID:        staff.PersonID,
		StaffNotes:      staff.StaffNotes,
		IsTeacher:       isTeacher,
		WasPresentToday: wasPresentToday,
		WorkStatus:      workStatus,
		AbsenceType:     absenceType,
		AccountRole:     accountRole,
		EmploymentType:  staff.EmploymentType,
		CreatedAt:       staff.CreatedAt,
		UpdatedAt:       staff.UpdatedAt,
	}

	if staff.Person != nil {
		response.Person = newPersonResponse(staff.Person, email, avatar)
	}

	if !showPersonnel {
		redactStaffPersonnelFields(&response)
	}

	return response
}

// redactStaffPersonnelFields strips the personnel-file fields from a staff
// response, leaving the minimal colleague view (#2906). Keep the field list
// in sync with StaffResponse — a new field is visible to every colleague
// until it is listed here.
func redactStaffPersonnelFields(response *StaffResponse) {
	response.StaffNotes = ""
	response.AbsenceType = ""
	response.AbsenceTypeLabel = ""
	response.EmploymentType = nil
	if response.Person != nil {
		response.Person.TagID = ""
	}
}

// newTeacherResponse creates a teacher response
func newTeacherResponse(ctx context.Context, staff *users.Staff, teacher *users.Teacher, wasPresentToday bool, workStatus string, absenceType string, accountRole string, email string, avatar string) TeacherResponse {
	return buildTeacherResponse(canSeeStaffPersonnelFields(ctx), staff, teacher, wasPresentToday, workStatus, absenceType, accountRole, email, avatar)
}

// buildTeacherResponse is newTeacherResponse with the personnel decision
// already made — see buildStaffResponse.
func buildTeacherResponse(showPersonnel bool, staff *users.Staff, teacher *users.Teacher, wasPresentToday bool, workStatus string, absenceType string, accountRole string, email string, avatar string) TeacherResponse {
	staffResponse := buildStaffResponse(showPersonnel, staff, true, wasPresentToday, workStatus, absenceType, accountRole, email, avatar)

	response := TeacherResponse{
		StaffResponse:  staffResponse,
		TeacherID:      teacher.ID,
		Specialization: strings.TrimSpace(teacher.Specialization),
		Role:           teacher.Role,
		Qualifications: teacher.Qualifications,
	}

	// Free-text qualifications are HR-file data; Specialization and Role are
	// the pedagogical labels the group and substitution screens display.
	if !showPersonnel {
		response.Qualifications = ""
	}

	return response
}

// updateStaffResponseFor maps the teacher-record outcome of the update to the
// endpoint's historical response/message contract.
func updateStaffResponseFor(ctx context.Context, staff *users.Staff, teacher *users.Teacher, action usersSvc.TeacherAction) (interface{}, string) {
	switch action {
	case usersSvc.TeacherActionUpdated, usersSvc.TeacherActionCreated, usersSvc.TeacherActionExisting:
		return newTeacherResponse(ctx, staff, teacher, false, "", "", "", "", ""), "Teacher updated successfully"
	case usersSvc.TeacherActionUpdateFailed:
		return newStaffResponse(ctx, staff, false, false, "", "", "", "", ""), "Staff member updated successfully, but failed to update teacher record"
	case usersSvc.TeacherActionCreateFailed:
		return newStaffResponse(ctx, staff, false, false, "", "", "", "", ""), "Staff member updated successfully, but failed to create teacher record"
	default:
		return newStaffResponse(ctx, staff, false, false, "", "", "", "", ""), "Staff member updated successfully"
	}
}

// StaffWithRoleResponse represents a staff member with role information
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

// ScheduleEntryRequest represents a single day in the schedule
type ScheduleEntryRequest struct {
	WeekIndex     int     `json:"week_index"`
	DayOfWeek     int     `json:"day_of_week"`
	TargetMinutes int     `json:"target_minutes"`
	StartTime     *string `json:"start_time,omitempty"`
}

// ScheduleEntryResponse represents a single day in the schedule response
type ScheduleEntryResponse struct {
	WeekIndex     int     `json:"week_index"`
	DayOfWeek     int     `json:"day_of_week"`
	TargetMinutes int     `json:"target_minutes"`
	StartTime     *string `json:"start_time,omitempty"`
}

// ScheduleModelInfo describes the assigned work-time template, when there is one.
type ScheduleModelInfo struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	RotationLength     int    `json:"rotation_length"`
	RotationAnchorDate string `json:"rotation_anchor_date"`
}

// ScheduleResponse wraps the resolved schedule with rotation metadata.
type ScheduleResponse struct {
	Mode               string                  `json:"mode"`
	Model              *ScheduleModelInfo      `json:"model,omitempty"`
	RotationLength     int                     `json:"rotation_length"`
	RotationAnchorDate string                  `json:"rotation_anchor_date"`
	Entries            []ScheduleEntryResponse `json:"entries"`
	WeeklyTotals       []int                   `json:"weekly_totals"`
	ValidFrom          string                  `json:"valid_from,omitempty"`
}

// scheduleUpdateRequest is the union body for PUT /api/staff/{id}/schedule.
//
// Mode "template": only ModelID is required. Existing custom entries are
// archived and the staff is bound to the template.
// Mode "custom":   RotationLength + Entries describe a per-staff pattern.
//
//	Entries that exceed the rotation are rejected.
//	SaveAsTemplateName is optional; when set we additionally
//	create a tenant template from the same payload and bind it.
type scheduleUpdateRequest struct {
	Mode               string                 `json:"mode"`
	ModelID            *int64                 `json:"model_id,omitempty"`
	RotationLength     int                    `json:"rotation_length,omitempty"`
	RotationAnchorDate string                 `json:"rotation_anchor_date,omitempty"`
	Entries            []ScheduleEntryRequest `json:"entries"`
	SaveAsTemplateName string                 `json:"save_as_template,omitempty"`
}

// toServiceInput maps the api request DTO to the service-layer input struct,
// keeping api types out of services/active.
func (req scheduleUpdateRequest) toServiceInput() activeSvc.ScheduleUpdateInput {
	entries := make([]activeSvc.ScheduleEntry, 0, len(req.Entries))
	for _, e := range req.Entries {
		entries = append(entries, activeSvc.ScheduleEntry{
			WeekIndex:     e.WeekIndex,
			DayOfWeek:     e.DayOfWeek,
			TargetMinutes: e.TargetMinutes,
			StartTime:     e.StartTime,
		})
	}
	return activeSvc.ScheduleUpdateInput{
		Mode:               req.Mode,
		ModelID:            req.ModelID,
		RotationLength:     req.RotationLength,
		RotationAnchorDate: req.RotationAnchorDate,
		Entries:            entries,
		SaveAsTemplateName: req.SaveAsTemplateName,
	}
}
