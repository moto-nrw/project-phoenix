package securityruntime

import (
	"github.com/moto-nrw/project-phoenix/modules/securityruntime/internal/application/authorize"
	securitycontract "github.com/moto-nrw/project-phoenix/modules/securityruntime/ports"
)

// securitycontract.StudentNoteReader is the caller as the note audience needs them: their
// groups, activities and normalized school classes, supplied rather than
// fetched.

// securitycontract.StudentNoteChild is the child side of the note audience question.

// securitycontract.StudentNoteAudience is how far into a child's note card a caller reaches,
// expressed as facts rather than as the note owner's visibility values.

// ResolveStudentNoteAudience answers how far into a child's note card this
// caller reaches. It assumes the caller already passed the student read gate.
func ResolveStudentNoteAudience(permissions []string, reader securitycontract.StudentNoteReader, child securitycontract.StudentNoteChild) securitycontract.StudentNoteAudience {
	return authorize.ResolveStudentNoteAudience(permissions, reader, child)
}

// CanDeleteStudentNote reports whether the caller may remove somebody's note:
// the administration, or the leadership of the group the note refers to.
func CanDeleteStudentNote(audience securitycontract.StudentNoteAudience, activityGroupID, educationGroupID, childGroupID *int64) bool {
	return authorize.CanDeleteStudentNote(audience, activityGroupID, educationGroupID, childGroupID)
}
