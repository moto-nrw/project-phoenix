package education

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// ClassTeacher assigns a staff member to a school class (#1772). The class is
// the free-text users.students.school_class value, matched case-insensitively
// on LOWER(BTRIM(...)) — there is deliberately no class entity: assignments
// are redone every school year anyway, and the grade-transition tooling
// rewrites the student strings. SchoolClass stores the display form as
// entered; comparisons must go through schoolclass.Normalize.
type ClassTeacher struct {
	base.Model `bun:"schema:education,table:class_teachers"`
	base.TenantModel
	StaffID     int64  `bun:"staff_id,notnull" json:"staff_id"`
	SchoolClass string `bun:"school_class,notnull" json:"school_class"`
}

// Validate ensures class teacher data is valid
func (ct *ClassTeacher) Validate() error {
	if ct.StaffID <= 0 {
		return errors.New("staff ID is required")
	}

	if strings.TrimSpace(ct.SchoolClass) == "" {
		return errors.New("school class is required")
	}

	return nil
}
