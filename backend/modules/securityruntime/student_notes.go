package securityruntime

import "github.com/moto-nrw/project-phoenix/auth/authorize"

// StudentNoteReader is the caller as the note audience needs them: their
// groups, activities and normalized school classes, supplied rather than
// fetched.
type StudentNoteReader = authorize.StudentNoteReader

// StudentNoteChild is the child side of the note audience question.
type StudentNoteChild = authorize.StudentNoteChild

// StudentNoteAudience is how far into a child's note card a caller reaches,
// expressed as facts rather than as the note owner's visibility values.
type StudentNoteAudience = authorize.StudentNoteAudience

// ResolveStudentNoteAudience answers how far into a child's note card this
// caller reaches. It assumes the caller already passed the student read gate.
func ResolveStudentNoteAudience(permissions []string, reader StudentNoteReader, child StudentNoteChild) StudentNoteAudience {
	return authorize.ResolveStudentNoteAudience(permissions, reader, child)
}

// CanDeleteStudentNote reports whether the caller may remove somebody's note:
// the administration, or the leadership of the group the note refers to.
func CanDeleteStudentNote(audience StudentNoteAudience, activityGroupID, educationGroupID, childGroupID *int64) bool {
	return authorize.CanDeleteStudentNote(audience, activityGroupID, educationGroupID, childGroupID)
}
