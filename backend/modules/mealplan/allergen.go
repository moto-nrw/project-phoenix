package mealplan

import "slices"

// Allergen is the code of one of the 14 main allergens a dish can be
// marked with. The list is fixed in code, not per school (#4777); the
// database CHECK of schedule.meal_plan_entries.allergens holds the same
// codes.
type Allergen string

const (
	AllergenGluten      Allergen = "gluten"
	AllergenCrustaceans Allergen = "crustaceans"
	AllergenEggs        Allergen = "eggs"
	AllergenFish        Allergen = "fish"
	AllergenPeanuts     Allergen = "peanuts"
	AllergenSoybeans    Allergen = "soybeans"
	AllergenMilk        Allergen = "milk"
	AllergenNuts        Allergen = "nuts"
	AllergenCelery      Allergen = "celery"
	AllergenMustard     Allergen = "mustard"
	AllergenSesame      Allergen = "sesame"
	AllergenSulphites   Allergen = "sulphites"
	AllergenLupin       Allergen = "lupin"
	AllergenMolluscs    Allergen = "molluscs"
)

// Allergens lists the main allergens in their display order.
func Allergens() []Allergen {
	return []Allergen{
		AllergenGluten, AllergenCrustaceans, AllergenEggs, AllergenFish, AllergenPeanuts,
		AllergenSoybeans, AllergenMilk, AllergenNuts, AllergenCelery, AllergenMustard,
		AllergenSesame, AllergenSulphites, AllergenLupin, AllergenMolluscs,
	}
}

// NormalizeAllergens returns the codes once each, in display order. ok is
// false when a code is not one of the main allergens.
func NormalizeAllergens(codes []Allergen) (normalized []Allergen, ok bool) {
	normalized = make([]Allergen, 0, len(codes))
	for _, allergen := range Allergens() {
		if slices.Contains(codes, allergen) {
			normalized = append(normalized, allergen)
		}
	}
	for _, code := range codes {
		if !slices.Contains(normalized, code) {
			return nil, false
		}
	}
	return normalized, true
}

// AllergenCodes returns the entry's allergens as plain codes, an empty list
// when the dish is unmarked.
func (e Entry) AllergenCodes() []string {
	codes := make([]string, 0, len(e.Allergens))
	for _, allergen := range e.Allergens {
		codes = append(codes, string(allergen))
	}
	return codes
}
