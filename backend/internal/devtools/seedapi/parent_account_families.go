package api

// parentAccountGuardianIndexes are the DemoGuardians who get a parent-portal
// account, in the order of rt.Parents: both parents of Felix Schneider first,
// then one parent each of four other children.
var parentAccountGuardianIndexes = []int{0, 1, 5, 19, 32, 54}

// sickDemoCandidates are the DemoStudents indexes the seed may report sick:
// the children of every second group (DemoStudents lists ten per group), so
// about one child in twenty is ill, and none with a parent account: the
// parents portal, the product screenshots and the demo show those.
func sickDemoCandidates() []int {
	portal := make(map[int]bool, len(parentAccountGuardianIndexes))
	for _, guardian := range parentAccountGuardianIndexes {
		portal[DemoGuardians[guardian].StudentIndex] = true
	}
	candidates := make([]int, 0, len(DemoStudents))
	for index := range DemoStudents {
		if !portal[index] && index%20 < 10 {
			candidates = append(candidates, index)
		}
	}
	return candidates
}

// requestParent returns parents[index] when it exists and has a child, else
// the first parent. The seeded parent requests go to several families, so the
// OGS request inbox lists more than one child.
func requestParent(parents []ParentCredentials, index int) ParentCredentials {
	if index < len(parents) && len(parents[index].StudentIDs) > 0 {
		return parents[index]
	}
	return parents[0]
}

// coGuardianFirstName names the second guardian an enrollment form lists. The
// family name comes from the form; the first name varies per family.
func coGuardianFirstName(familyName string) string {
	names := []string{"Jan", "Nina", "Tobias", "Miriam", "Stefan", "Laura", "Markus", "Sandra"}
	return names[len([]rune(familyName))%len(names)]
}
