package users

type CaregiverCapabilityBlockerCode string

const (
	CaregiverCapabilityBlockerMissingUsableRole        CaregiverCapabilityBlockerCode = "missing_usable_role"
	CaregiverCapabilityBlockerActiveGroupSupervisions  CaregiverCapabilityBlockerCode = "active_group_supervisions"
	CaregiverCapabilityBlockerActiveGroupSubstitutions CaregiverCapabilityBlockerCode = "active_group_substitutions"
	CaregiverCapabilityBlockerActivitySupervisions     CaregiverCapabilityBlockerCode = "activity_supervisions"
	CaregiverCapabilityBlockerGroupAssignments         CaregiverCapabilityBlockerCode = "group_assignments"
)

// CaregiverCapabilityState captures whether an account can currently act as an
// operational caregiver inside a tenant.
type CaregiverCapabilityState struct {
	AccountID int64  `json:"account_id"`
	Email     string `json:"email"`

	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`

	PersonID  *int64 `json:"person_id,omitempty"`
	StaffID   *int64 `json:"staff_id,omitempty"`
	TeacherID *int64 `json:"teacher_id,omitempty"`

	HasAdminRole bool `json:"has_admin_role"`
	HasUserRole  bool `json:"has_user_role"`

	HasPerson           bool `json:"has_person"`
	HasStaff            bool `json:"has_staff"`
	HasTeacher          bool `json:"has_teacher"`
	HasCaregiverProfile bool `json:"has_caregiver_profile"`
	IsActiveCaregiver   bool `json:"is_active_caregiver"`
	DisableBlocked      bool `json:"disable_blocked"`

	DisableBlockers []CaregiverCapabilityBlockerCode `json:"disable_blockers,omitempty"`

	ActiveSupervisions   []BlockerSupervision  `json:"active_supervisions,omitempty"`
	ActiveSubstitutions  []BlockerSubstitution `json:"active_substitutions,omitempty"`
	ActivitySupervisions []BlockerActivity     `json:"activity_supervisions,omitempty"`
	GroupAssignments     []BlockerGroup        `json:"group_assignments,omitempty"`
}

// BlockerSupervision represents an active group supervision that blocks disable.
type BlockerSupervision struct {
	ID        int64  `json:"id"`
	GroupID   int64  `json:"-"`
	GroupName string `json:"group_name"`
	StartDate string `json:"start_date"`
}

// BlockerSubstitution represents an active group handover that blocks disable.
type BlockerSubstitution struct {
	ID        int64  `json:"id"`
	GroupName string `json:"group_name"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// BlockerActivity represents an activity supervision assignment that blocks disable.
type BlockerActivity struct {
	ID           int64  `json:"id"`
	ActivityID   int64  `json:"activity_id"`
	ActivityName string `json:"activity_name"`
	IsPrimary    bool   `json:"is_primary"`
}

// BlockerGroup represents a group-teacher assignment that blocks disable.
type BlockerGroup struct {
	ID         int64   `json:"id"`
	GroupID    int64   `json:"group_id"`
	GroupName  string  `json:"group_name"`
	TeacherID  int64   `json:"teacher_id"`
	TeacherIDs []int64 `bun:"teacher_ids,array" json:"teacher_ids,omitempty"`
}

// EnableCaregiverCapabilityInput supplies missing profile data when an existing
// account is activated as a caregiver.
type EnableCaregiverCapabilityInput struct {
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Position  string `json:"position,omitempty"`
}
