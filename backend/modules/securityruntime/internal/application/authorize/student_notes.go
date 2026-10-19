package authorize

import (
	"slices"

	securitycontract "github.com/moto-nrw/project-phoenix/modules/securityruntime/ports"
)

// securitycontract.StudentNoteReader is the caller as the note audience needs them. The facts
// are supplied, not fetched: the caller's groups, activities and school classes
// live with three other owners, and a policy that reached for them itself
// would drag their packages into this one.

// securitycontract.StudentNoteChild is the child side of the audience question.

// securitycontract.StudentNoteAudience is the decision, deliberately expressed as facts rather
// than as the note owner's visibility values.
//
// This package must not name those values: it cannot import the owner that
// defines them, so naming them here would mean maintaining a second copy of an
// enum whose drift nothing could catch. The caller — which holds both sides —
// turns these facts into the owner's filter.

// ResolveStudentNoteAudience answers how far into a child's note card this
// caller reaches. It assumes the caller already passed CanReadStudent: every
// reader who gets this far sees the team-wide notes, which is today's
// unrestricted scope for child data and therefore changes nothing for anyone.
//
// The two narrower audiences are new, and they are the first place in this
// system where a group decides what a staff member sees about a child —
// CONTEXT.md otherwise states the opposite. That is deliberate and bounded to
// notes: a note is somebody's account of a situation, and the school decides
// per note who was part of it.
//
//   - CareTeam: the caller has this child, through their own OGS group, their
//     own school class, or an activity the child is enrolled in.
//   - The led group IDs: resolved per note by the store, so a leadership note
//     reaches exactly the leadership of the group it refers to.
func ResolveStudentNoteAudience(
	userPermissions []string,
	reader securitycontract.StudentNoteReader,
	child securitycontract.StudentNoteChild,
) securitycontract.StudentNoteAudience {
	if HasAdminWildcard(userPermissions) {
		return securitycontract.StudentNoteAudience{Admin: true, CareTeam: true}
	}
	return securitycontract.StudentNoteAudience{
		CareTeam:             readerHasChild(reader, child),
		LedEducationGroupIDs: reader.LedEducationGroupIDs,
		LedActivityGroupIDs:  reader.ActivityGroupIDs,
	}
}

// readerHasChild reports whether this caller's assignments cover the child.
func readerHasChild(reader securitycontract.StudentNoteReader, child securitycontract.StudentNoteChild) bool {
	if child.GroupID != nil && slices.Contains(reader.GroupIDs, *child.GroupID) {
		return true
	}
	for _, enrolled := range child.ActivityGroupIDs {
		if slices.Contains(reader.ActivityGroupIDs, enrolled) {
			return true
		}
	}
	if child.SchoolClass == "" {
		return false
	}
	return slices.Contains(reader.Classes, child.SchoolClass)
}

// CanDeleteStudentNote reports whether the caller may remove somebody's note.
//
// Removing is a leadership act, not an authoring one: the school's
// administration, or the staff member who leads the group or activity the note
// refers to. Authorship deliberately does not appear here — a Klassenbuch in
// which the author of an incident note can make it disappear is not a record.
// The author's path is the correction, which lives with the note owner.
//
// A note without a group reference falls back to the leadership of the child's
// own OGS group: a general note still belongs to somebody's group, and leaving
// it deletable by administrators alone would strand every school that does not
// hand out admin rights.
func CanDeleteStudentNote(
	audience securitycontract.StudentNoteAudience,
	activityGroupID, educationGroupID, childGroupID *int64,
) bool {
	if audience.Admin {
		return true
	}
	if activityGroupID != nil {
		return slices.Contains(audience.LedActivityGroupIDs, *activityGroupID)
	}
	if educationGroupID != nil {
		return slices.Contains(audience.LedEducationGroupIDs, *educationGroupID)
	}
	return childGroupID != nil && slices.Contains(audience.LedEducationGroupIDs, *childGroupID)
}
