package importpkg

// StaffImportRow represents a single row of staff (Mitarbeiter) import data.
//
// Staff are onboarded by bulk-creating invitations: the import never sets a
// password or PIN. Each row becomes an invitation; the Person/Account/Staff/
// Teacher records are created when the invitee accepts and sets their own
// password. See services/import/staff_import_config.go.
type StaffImportRow struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	RoleName  string `json:"role_name"`          // Human-readable role (validated against tenant roles)
	Position  string `json:"position,omitempty"` // Optional job title (maps to teacher.role on accept)

	// RoleID is resolved from RoleName during validation; not present in the CSV.
	RoleID int64 `json:"-"`
}
