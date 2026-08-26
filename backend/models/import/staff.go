package importpkg

// StaffImportRow represents one Mitarbeiter row of the staff import.
//
// The row is a full Stammdatensatz (#2600): the import creates Person, Staff,
// the optional caregiver profile and the master data immediately. An e-mail
// address additionally issues a portal invitation; without one the person is
// a directory entry without a login, which is a legitimate state for staff
// (Honorarkräfte, Praktikanten, Küchenpersonal).
type StaffImportRow struct {
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Email      string `json:"email,omitempty"`    // Login e-mail; empty = no invitation
	RoleName   string `json:"role_name"`          // Human-readable role (validated against tenant roles)
	Position   string `json:"position,omitempty"` // Optional job title (maps to teacher.role)
	Birthday   string `json:"birthday,omitempty"` // YYYY-MM-DD (also accepts DD.MM.YYYY / DD.MM.YY)
	Gender     string `json:"gender,omitempty"`   // female / male / diverse (German aliases accepted)
	StaffNotes string `json:"staff_notes,omitempty"`

	// Payroll identity and contract (users.staff + users.staff_master_data)
	PersonnelNumber  string `json:"personnel_number,omitempty"`
	EmploymentType   string `json:"employment_type,omitempty"` // full_time / part_time / minijob
	EntryDate        string `json:"entry_date,omitempty"`
	ContractEndDate  string `json:"contract_end_date,omitempty"`
	ProbationEndDate string `json:"probation_end_date,omitempty"`
	WeeklyHours      string `json:"weekly_hours,omitempty"`

	// Contact (users.staff_master_data)
	AddressStreet         string `json:"address_street,omitempty"`
	AddressPostalCode     string `json:"address_postal_code,omitempty"`
	AddressCity           string `json:"address_city,omitempty"`
	Phone                 string `json:"phone,omitempty"`
	ContactEmail          string `json:"contact_email,omitempty"`
	EmergencyContactName  string `json:"emergency_contact_name,omitempty"`
	EmergencyContactPhone string `json:"emergency_contact_phone,omitempty"`

	// Qualifications: "Erste Hilfe (2024-03-01 bis 2026-03-01); Schwimmschein"
	Qualifications string `json:"qualifications,omitempty"`

	// Resolved during validation
	RoleID int64 `json:"-"`

	// Column presence, so update mode can tell "cell left empty" from
	// "column not in the file". Only columns that clear a value on an
	// explicit empty cell need this; free-text columns keep the old value
	// when the cell is blank.
	HasQualificationsColumn bool `json:"-"`
}
