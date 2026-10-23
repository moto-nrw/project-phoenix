package architecture

import "fmt"

// renamedDataObjectLoosenings judges a data object a new candidate migration
// renames (ALTER TABLE ... RENAME TO). The rename keeps the rows, so it keeps
// the owner: the old name must be owned at the base, must leave the policy,
// and the new name must carry the same write owner. Anything else is the
// ordinary new assignment it would be without the rename (ADR 0090).
func renamedDataObjectLoosenings(baseObjects, candidateObjects map[string]DataObject, name, previous string) []string {
	current := candidateObjects[name]
	before, owned := baseObjects[previous]
	switch {
	case !owned:
		return []string{fmt.Sprintf("data object %s was newly assigned to owner %s (renamed from %s, which the base policy does not own)", name, current.WriteOwner, previous)}
	case current.WriteOwner != before.WriteOwner:
		return []string{fmt.Sprintf("data object %s renamed from %s changed write owner from %s to %s", name, previous, before.WriteOwner, current.WriteOwner)}
	}
	if _, kept := candidateObjects[previous]; kept {
		return []string{fmt.Sprintf("data object %s renamed from %s keeps the old name in the policy", name, previous)}
	}
	return nil
}
