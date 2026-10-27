package api

import "fmt"

// demoKoelnStreets are streets of the Cologne quarters around the demo
// school; the phone numbers of the demo use Cologne's 0221 too.
var demoKoelnStreets = []struct{ street, postalCode string }{
	{"Neusser Straße", "50733"},
	{"Merheimer Straße", "50733"},
	{"Venloer Straße", "50823"},
	{"Subbelrather Straße", "50823"},
	{"Zülpicher Straße", "50937"},
	{"Luxemburger Straße", "50939"},
	{"Berliner Straße", "51063"},
	{"Bergisch Gladbacher Straße", "51065"},
}

// demoKoelnAddress is the home address of the family of the child with the
// given index: a child and its guardians share it.
func demoKoelnAddress(childIndex int) (street, postalCode, city string) {
	entry := demoKoelnStreets[childIndex%len(demoKoelnStreets)]
	return fmt.Sprintf("%s %d", entry.street, 3+(childIndex*7)%120), entry.postalCode, "Köln"
}
