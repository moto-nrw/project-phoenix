package schoolstructure

import (
	"context"
	"errors"
)

var (
	ErrInvalidStaff = errors.New("invalid staff")
	ErrInvalidDay   = errors.New("invalid day")
)

// SubstitutedGroup is a group a staff member substitutes for on one day.
type SubstitutedGroup struct {
	Group Group `json:"group"`
	// ViaSubstitution is true when the regular staff slot of an active
	// substitution is unassigned: the substitute is then the group's only
	// access path, which is what the navigation context flags.
	ViaSubstitution bool `json:"via_substitution"`
}

// StaffGroupQuery answers which groups and school classes belong to one
// staff member. A caller's groups are the union of its teacher groups
// (ListGroupsByTeacher) and the groups it substitutes for today
// (ListSubstitutedGroups). Every read runs on the caller's ambient tenant
// transaction, so a teacher or staff id of another tenant resolves to an
// empty result.
type StaffGroupQuery interface {
	// ListGroupsByTeacher returns the groups assigned to the teacher, sorted
	// by name.
	ListGroupsByTeacher(ctx context.Context, teacherID int64) ([]Group, error)
	// ListSubstitutedGroups returns one entry per group the staff member
	// substitutes for on the calendar day, sorted by name. The day is a
	// timezone.Date in its YYYY-MM-DD form (day.String()); the public
	// contract cannot reference the shared-kernel type (#3499). A caller
	// merging these with its teacher groups applies ViaSubstitution to a
	// teacher group it also substitutes for, like GetSubstitutedGroupIDs.
	ListSubstitutedGroups(ctx context.Context, staffID int64, day string) ([]SubstitutedGroup, error)
	// ListSchoolClassesByStaff returns the school classes assigned to the
	// staff member via education.class_teachers (#1772) as display strings
	// in class order.
	ListSchoolClassesByStaff(ctx context.Context, staffID int64) ([]string, error)
}
