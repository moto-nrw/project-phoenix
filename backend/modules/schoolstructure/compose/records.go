package compose

import "github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"

// The School Structure values the group service's and the substitution
// module's ports exchange. The legacy composition root binds those ports over
// the retained repositories and the foreign owners' capabilities and names
// the values through these aliases; models/education is gone (#3556). None of
// them carries an ORM mapping: the group store keeps its rows private.

type (
	Group              = domain.Group
	RoomProjection     = domain.GroupRoom
	GroupListQuery     = domain.GroupListQuery
	StaffGroupID       = domain.StaffGroupID
	GroupTeacher       = domain.GroupTeacher
	ClassTeacher       = domain.ClassTeacher
	GroupSubstitution  = domain.GroupSubstitution
	SubstitutionStaff  = domain.SubstitutionStaff
	Caregiver          = domain.Caregiver
	HandoverQuery      = domain.HandoverQuery
	SchoolClassChange  = domain.SchoolClassChange
	SubstitutionChange = domain.SubstitutionChange
)

const (
	SubstitutionAssigned = domain.SubstitutionAssigned
	SubstitutionEnded    = domain.SubstitutionEnded
)

// ErrHandoverExists reports a group handover that duplicates a stored one.
var ErrHandoverExists = domain.ErrHandoverExists
