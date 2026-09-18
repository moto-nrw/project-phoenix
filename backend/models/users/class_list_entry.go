package users

import (
	"github.com/moto-nrw/project-phoenix/models/base"
)

// ClassListEntry is a child of the class cohort WITHOUT an OGS record
// (#2382): only first name, last name and the free-text school class. It
// exists so class lists and the Lehrkraft class-day view can show the
// complete Klassenverband ("Keine Betreuung") — it is deliberately NOT a
// student: no person, no guardians, no attendance, no care planning can ever
// reference it, and it must never surface in the student search, the parent
// portal or any planning view. SchoolClass stores the display form as
// entered; comparisons go through schoolclass.Normalize like every other
// class string (see models/education.ClassTeacher).
//
// The rows belong to the School Membership owner (#2668): this struct is the
// bun mapping test fixtures insert through, nothing more. Validation, the
// audit display form and the unique-index contract live with that owner.
type ClassListEntry struct {
	base.Model `bun:"schema:users,table:class_list_entries"`
	base.TenantModel
	FirstName   string `bun:"first_name,notnull" json:"first_name"`
	LastName    string `bun:"last_name,notnull" json:"last_name"`
	SchoolClass string `bun:"school_class,notnull" json:"school_class"`
	// CreatedBy is the creating account ID; nil for rows created by system
	// paths without an authenticated account.
	CreatedBy *int64 `bun:"created_by" json:"created_by,omitempty"`
}
