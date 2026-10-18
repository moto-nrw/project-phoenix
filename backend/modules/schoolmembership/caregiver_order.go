package schoolmembership

import (
	"sort"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// SortActiveCaregivers puts caregivers into the picker order: first name,
// last name, then staff ID, compared in German dictionary order ignoring
// case, so "Özlem" sorts next to "Otto", not after "Zimmermann". It keeps
// the order of the query the picker replaced, which ran under the database
// collation (en_US.utf8).
func SortActiveCaregivers(caregivers []ActiveCaregiver) {
	// One collator per call: it is not safe for concurrent use.
	collator := collate.New(language.German, collate.IgnoreCase)
	sort.SliceStable(caregivers, func(i, j int) bool {
		if c := collator.CompareString(caregivers[i].FirstName, caregivers[j].FirstName); c != 0 {
			return c < 0
		}
		if c := collator.CompareString(caregivers[i].LastName, caregivers[j].LastName); c != 0 {
			return c < 0
		}
		return caregivers[i].StaffID < caregivers[j].StaffID
	})
}
