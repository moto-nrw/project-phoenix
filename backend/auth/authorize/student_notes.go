package authorize

import "slices"

// StudentNoteReader is the caller as the note audience needs them. The facts
// are supplied, not fetched: the caller's groups, activities and school classes
// live with three other owners, and a policy that reached for them itself
// would drag their packages into this one.
type StudentNoteReader struct {
	// GroupIDs are the caller's own and substituted OGS groups.
	GroupIDs []int64
	// LedEducationGroupIDs are assigned directly to the caller's teacher
	// profile. Unlike GroupIDs, temporary substitutions never confer the
	// authority to read or remove a leadership note.
	LedEducationGroupIDs []int64
	// ActivityGroupIDs are the activities the caller supervises.
	ActivityGroupIDs []int64
	// Classes are the caller's school classes, already normalized with
	// schoolclass.Normalize. Normalizing here would mean owning the rule a
	// second time; comparing raw strings would silently miss " 3A " against
	// "3a".
	Classes []string
}

// StudentNoteChild is the child side of the audience question.
type StudentNoteChild struct {
	GroupID *int64
	// SchoolClass is normalized like StudentNoteReader.Classes.
	SchoolClass string
	// ActivityGroupIDs are the activities the child is enrolled in. Empty means
	// the child takes part in none — a lookup that failed is the caller's error
	// to raise, not an empty audience to render.
	ActivityGroupIDs []int64
}

// StudentNoteAudience is the decision, deliberately expressed as facts rather
// than as the note owner's visibility values.
//
// This package must not name those values: it cannot import the owner that
// defines them, so naming them here would mean maintaining a second copy of an
// enum whose drift nothing could catch. The caller — which holds both sides —
// turns these facts into the owner's filter.
type StudentNoteAudience struct {
	// Admin reads every note of the child, whatever its audience.
	Admin bool
	// CareTeam says the caller actually has this child.
	CareTeam bool
	// LedActivityGroupIDs and LedEducationGroupIDs are the groups the caller
	// leads; they unlock the leadership notes that refer to them, and they are
	// what the delete decision below is measured against.
	LedActivityGroupIDs  []int64
	LedEducationGroupIDs []int64
}

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
	reader StudentNoteReader,
	child StudentNoteChild,
) StudentNoteAudience {
	if HasAdminWildcard(userPermissions) {
		return StudentNoteAudience{Admin: true, CareTeam: true}
	}
	return StudentNoteAudience{
		CareTeam:             readerHasChild(reader, child),
		LedEducationGroupIDs: reader.LedEducationGroupIDs,
		LedActivityGroupIDs:  reader.ActivityGroupIDs,
	}
}

// readerHasChild reports whether this caller's assignments cover the child.
func readerHasChild(reader StudentNoteReader, child StudentNoteChild) bool {
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
	audience StudentNoteAudience,
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
