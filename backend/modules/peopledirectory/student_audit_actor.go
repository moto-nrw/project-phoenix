package peopledirectory

import "context"

// StudentAuditActorCommand resolves the authenticated editor at the original
// owner boundary. Nil snapshots retain the legacy no-op behavior.
type StudentAuditActorCommand interface {
	RecordStudentChangesForActor(context.Context, *StudentAuditSnapshot, *StudentAuditSnapshot, int64) error
}
