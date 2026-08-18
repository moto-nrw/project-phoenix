package users

import (
	"errors"
	"strings"

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
// ClassListEntryUniqueIndexName is the unique index behind the "one child
// once per class" rule (tenant, LOWER(BTRIM(first/last/class))). The service
// maps a 23505 on it to ErrClassListEntryDuplicate so a concurrent create
// loses with the documented duplicate response instead of a 500.
const ClassListEntryUniqueIndexName = "uniq_class_list_entries_name_class"

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

// Validate ensures class list entry data is valid.
func (e *ClassListEntry) Validate() error {
	if strings.TrimSpace(e.FirstName) == "" {
		return errors.New("first name is required")
	}
	if strings.TrimSpace(e.LastName) == "" {
		return errors.New("last name is required")
	}
	if strings.TrimSpace(e.SchoolClass) == "" {
		return errors.New("school class is required")
	}
	return nil
}

// DisplayValue renders the audit-trail representation of the entry:
// "Vorname Nachname (Klasse)".
func (e *ClassListEntry) DisplayValue() string {
	return strings.TrimSpace(e.FirstName) + " " + strings.TrimSpace(e.LastName) + " (" + strings.TrimSpace(e.SchoolClass) + ")"
}
