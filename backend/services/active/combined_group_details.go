package active

import "github.com/moto-nrw/project-phoenix/modules/studentpresence"

// CombinedGroupDetails joins a combination to its mapped room sessions.
type CombinedGroupDetails struct {
	studentpresence.CombinedGroup
	GroupMappings []CombinedGroupMapping
	ActiveGroups  []*studentpresence.LiveGroup
}

type CombinedGroupMapping struct {
	studentpresence.GroupMapping
	ActiveGroup *studentpresence.LiveGroup
}
