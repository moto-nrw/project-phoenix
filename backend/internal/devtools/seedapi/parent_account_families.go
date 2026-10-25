package api

import (
	"strconv"
	"strings"
)

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

// demoGuardianHasLimitedRole mirrors the roles seedGuardians (stammdaten.go)
// links the demo guardians with: from index 6 on, every tenth is pickup only
// and every eleventh has a custom role. Their app accounts lack the full
// parent rights, so they neither answer nor write.
func demoGuardianHasLimitedRole(index int) bool {
	return index >= 6 && (index%10 == 0 || index%11 == 0)
}

// respondingFamilies are the accounts that answer polls and declarations:
// every scripted parent but the first, whose family keeps the open forms for
// the parents portal, then three of four family app accounts. Together with
// the families that answer nothing, the results look like a school where
// about two thirds of the families answered.
func respondingFamilies(rt *Runtime) []ParentCredentials {
	out := make([]ParentCredentials, 0, len(rt.Parents)+len(rt.FamilyAccounts))
	if len(rt.Parents) > 1 {
		out = append(out, rt.Parents[1:]...)
	}
	for i, account := range rt.FamilyAccounts {
		if i%4 != 3 {
			out = append(out, account)
		}
	}
	return out
}

// firstFamilyChildren are the children of the first demo parent as the
// parent API names them. Every account that shares one of them leaves it
// unanswered.
func firstFamilyChildren(rt *Runtime) map[string]bool {
	children := map[string]bool{}
	if len(rt.Parents) == 0 {
		return children
	}
	for _, id := range rt.Parents[0].StudentIDs {
		children[strconv.FormatInt(id, 10)] = true
	}
	return children
}

// relatedAccountInvite is the invitation a parent sends to the child's
// grandmother. It stays open, so its contact card carries only the name.
func relatedAccountInvite(parent ParentCredentials) map[string]any {
	lastName := parent.Name
	if fields := strings.Fields(parent.Name); len(fields) > 0 {
		lastName = fields[len(fields)-1]
	}
	return map[string]any{
		"email":      "renate." + asciiMailPart(lastName) + "@mail.example",
		"first_name": "Renate",
		"last_name":  lastName,
	}
}

// asciiMailPart lowercases a name for an e-mail address and spells out the
// German umlauts.
func asciiMailPart(name string) string {
	replacer := strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss")
	var out strings.Builder
	for _, r := range replacer.Replace(strings.ToLower(name)) {
		if (r >= 'a' && r <= 'z') || r == '-' {
			out.WriteRune(r)
		}
	}
	return out.String()
}
