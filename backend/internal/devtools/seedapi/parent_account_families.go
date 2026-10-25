package api

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// parentAccountGuardianIndexes are the DemoGuardians who get a parent-portal
// account, in the order of rt.Parents: both parents of Felix Schneider first,
// then one parent each of four other families. Shirin Amini's entry is the
// one for Baran; her account also reaches his sister Aida.
var parentAccountGuardianIndexes = []int{0, 1, 6, 20, 35, 63}

// demoGuardianKey names a demo guardian. Siblings repeat their guardian in
// DemoGuardians, so the key, not the entry, identifies one person.
func demoGuardianKey(guardian DemoGuardian) string {
	return guardian.FirstName + " " + guardian.LastName
}

// demoGuardianFirstIndex is the index of the guardian's first entry.
func demoGuardianFirstIndex(guardian DemoGuardian) int {
	return slices.IndexFunc(DemoGuardians, func(entry DemoGuardian) bool {
		return demoGuardianKey(entry) == demoGuardianKey(guardian)
	})
}

// demoGuardianProfileCount is how many guardian profiles the seed creates:
// one per person, however many children the person is linked to.
func demoGuardianProfileCount() int {
	profiles := map[string]bool{}
	for _, guardian := range DemoGuardians {
		profiles[demoGuardianKey(guardian)] = true
	}
	return len(profiles)
}

// demoGuardianChildren returns the DemoStudents indexes the guardian of entry
// index is linked to: that entry's child first, then its siblings.
func demoGuardianChildren(index int) []int {
	guardian := DemoGuardians[index]
	children := []int{guardian.StudentIndex}
	for _, entry := range DemoGuardians {
		if demoGuardianKey(entry) == demoGuardianKey(guardian) && !slices.Contains(children, entry.StudentIndex) {
			children = append(children, entry.StudentIndex)
		}
	}
	return children
}

// demoGuardianStudentIDs returns the seeded IDs of demoGuardianChildren(index).
func demoGuardianStudentIDs(studentIDByIndex map[int]int64, index int) ([]int64, error) {
	children := demoGuardianChildren(index)
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		id, ok := studentIDByIndex[child]
		if !ok || id == 0 {
			return nil, fmt.Errorf("student index %d for guardian %s was not created", child, demoGuardianKey(DemoGuardians[index]))
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// demoGuardianHasAppAccount reports whether the guardian is one of the
// scripted parents with a parent-portal account.
func demoGuardianHasAppAccount(guardian DemoGuardian) bool {
	return slices.ContainsFunc(parentAccountGuardianIndexes, func(index int) bool {
		return demoGuardianKey(DemoGuardians[index]) == demoGuardianKey(guardian)
	})
}

// demoGuardianLink is the body that links one guardian entry to its child.
// Every tenth and eleventh guardian gets a narrower role. The role follows the
// person, so siblings see the same one, and the scripted parents keep the full
// role their parent-portal flows need.
func demoGuardianLink(guardianID int64, guardian DemoGuardian) map[string]any {
	link := map[string]any{
		"guardian_profile_id":  guardianID,
		"relationship_type":    guardian.Relationship,
		"is_primary":           guardian.IsPrimary,
		"is_emergency_contact": true,
		"can_pickup":           true,
		"emergency_priority":   1,
	}
	index := demoGuardianFirstIndex(guardian)
	switch {
	case index < 6 || demoGuardianHasAppAccount(guardian):
	case index%10 == 0:
		link["guardian_role"] = "pickup_only"
		link["relationship_type"] = "relative"
		link["is_emergency_contact"] = false
	case index%11 == 0:
		link["guardian_role"] = "custom"
		link["relationship_type"] = "other"
		link["is_emergency_contact"] = false
		link["can_pickup"] = false
	}
	if !guardian.IsPrimary {
		link["emergency_priority"] = 2
	}
	return link
}

// sickDemoCandidates are the DemoStudents indexes the seed may report sick:
// the children of every second group (DemoStudents lists ten per group), so
// about one child in twenty is ill, and none with a parent account: the
// parents portal, the product screenshots and the demo show those.
func sickDemoCandidates() []int {
	portal := make(map[int]bool, len(parentAccountGuardianIndexes))
	for _, guardian := range parentAccountGuardianIndexes {
		for _, child := range demoGuardianChildren(guardian) {
			portal[child] = true
		}
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

// enrollmentCoGuardians names the second guardian of the seeded enrollment
// families, by family name.
var enrollmentCoGuardians = map[string]string{
	"Çelik": "Murat", "Klein": "Laura", "Wolf": "Stefan", "Kowalczyk": "Tomasz",
	"Hartmann": "Jan", "Neumann": "Sandra", "Kovalenko": "Andrij", "Al-Hassan": "Khaled",
	"Vogt": "Stefan", "Kovac": "Kristina", "Schmitz": "Sandra", "Reimer": "Alexander",
	"Sommerfeld": "Tobias", "Sharma": "Robin", "Rossi": "Marco", "Winter": "Nina",
	"Popescu": "Mihai", "Smirnow": "Jelena", "Brandt": "Jens", "Öztürk": "Hakan",
	"Peters": "Markus", "Simon": "Laura", "Berisha": "Driton", "Amini": "Reza",
	"Nachtrag": "Jan",
}

// withDemoCoGuardian gives the form's co-guardian the email of the seeded
// profile of that person. Approval links a co-guardian by email, so the child
// gets the existing guardian instead of a second profile with the same name.
func withDemoCoGuardian(body map[string]any, email string) map[string]any {
	if guardians, ok := body["additional_guardians"].([]map[string]any); ok && len(guardians) > 0 {
		guardians[0]["email"] = email
	}
	return body
}

// coGuardianFirstName names the second guardian an enrollment form lists. The
// family name comes from the form; other families get a first name that
// varies per family.
func coGuardianFirstName(familyName string) string {
	if name, ok := enrollmentCoGuardians[familyName]; ok {
		return name
	}
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
