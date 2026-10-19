package ports

import "context"

type StudentAccessUserContext interface {
	HasCurrentStaff(ctx context.Context) (bool, error)
}

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

type StudentNoteChild struct {
	GroupID *int64
	// SchoolClass is normalized like StudentNoteReader.Classes.
	SchoolClass string
	// ActivityGroupIDs are the activities the child is enrolled in. Empty means
	// the child takes part in none — a lookup that failed is the caller's error
	// to raise, not an empty audience to render.
	ActivityGroupIDs []int64
}

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
