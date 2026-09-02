package active

import "context"

// CrossTenantStudent represents the minimal student data exposed
// when a hosting tenant requests information about a visiting student
// from another tenant. Only essential fields are included per GDPR
// data minimization (Datensparsamkeit) requirements.
//
// Used by the Ferienbetreuung (holiday care) feature where students
// from one school temporarily attend another school's program.
type CrossTenantStudent struct {
	StudentID    int64  `json:"student_id"`
	PersonID     int64  `json:"-"`
	HomeTenantID int64  `json:"-"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	GroupName    string `json:"group_name"`
	HomeTenant   string `json:"home_tenant"` // slug of the student's home school
}

// CrossTenantRepository lists the visiting students of a hosting tenant.
// The person names are resolved by the composition layer through the
// People Directory; the repository only yields the person reference.
type CrossTenantRepository interface {
	FindCrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]CrossTenantStudent, error)
}
