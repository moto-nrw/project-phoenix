package api

import (
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// The student seams of a graph that has no composed owner to hand them: test
// modules and the CLI roots. The serve root uses the *For constructors instead,
// so it keeps one observed People Directory rather than composing a second.

// NewStudentAudit composes an unobserved owner behind the change-history seam.
func NewStudentAudit(db transactionruntime.Database) *services.StudentAudit {
	return services.NewStudentAuditFor(services.MustNewPeopleDirectory(db.Pool()))
}
