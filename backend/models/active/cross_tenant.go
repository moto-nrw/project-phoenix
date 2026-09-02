package active

// CrossTenantStudent represents the minimal student data exposed
// when a hosting tenant requests information about a visiting student
// from another tenant. Only essential fields are included per GDPR
// data minimization (Datensparsamkeit) requirements.
//
// Used by the Ferienbetreuung (holiday care) feature where students
// from one school temporarily attend another school's program.
type CrossTenantStudent struct {
	StudentID    int64  `json:"student_id"`
	HomeTenantID int64  `json:"-"`
	GroupID      *int64 `json:"-"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	GroupName    string `json:"group_name"`
	HomeTenant   string `json:"home_tenant"` // slug of the student's home school
}
