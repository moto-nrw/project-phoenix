//go:build testhooks

package api

import (
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
)

// The student seams of a graph that has no composed owner to hand them: test
// modules and the CLI roots. The serve root uses the *For constructors instead,
// so it keeps one observed People Directory rather than composing a second.

// NewStudentAudit composes an unobserved owner behind the change-history seam.
func NewStudentAudit(db transactionruntime.Database) *owners.StudentAudit {
	return owners.NewStudentAuditFor(owners.MustNewPeopleDirectory(db))
}
