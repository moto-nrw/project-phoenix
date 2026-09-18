package repositories

import (
	auditRepositories "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	"github.com/uptrace/bun"
)

// The student seams of a graph that has no composed owner to hand them: test
// modules and the CLI roots. The serve root uses the *For constructors instead,
// so it keeps one observed People Directory rather than composing a second.

// NewStudentAudit composes an unobserved owner behind the change-history seam.
func NewStudentAudit(db *bun.DB) *StudentAudit {
	return NewStudentAuditFor(MustNewPeopleDirectory(db))
}

// NewStudentConsents composes unobserved owners behind the consent seam.
func NewStudentConsents(db *bun.DB) *StudentConsents {
	return NewStudentConsentsFor(
		MustNewPeopleDirectory(db),
		auditRepositories.NewStudentConsentChangeRepository(auditRootRuntime(db)),
	)
}
