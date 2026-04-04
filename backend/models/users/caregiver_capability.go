package users

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

	HasPerson            bool `json:"has_person"`
	HasStaff             bool `json:"has_staff"`
	HasTeacher           bool `json:"has_teacher"`
	HasCaregiverProfile  bool `json:"has_caregiver_profile"`
	IsActiveCaregiver    bool `json:"is_active_caregiver"`
	DisableBlocked       bool `json:"disable_blocked"`
	DisableBlockersCount int  `json:"disable_blockers_count"`

	DisableBlockers []string `json:"disable_blockers,omitempty"`
}

// EnableCaregiverCapabilityInput supplies missing profile data when an existing
// account is activated as a caregiver.
type EnableCaregiverCapabilityInput struct {
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Position  string `json:"position,omitempty"`
}
