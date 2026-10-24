// lib/meal-allergens.ts
// The 14 main allergens a dish of the meal plan can be marked with (#4777).
// The list is fixed in code; the backend (modules/mealplan/allergen.go) and
// the database CHECK hold the same codes. Display names live in the
// `mealAllergens` i18n namespace; the letters are the short marks with a
// legend, the same in every language.

export const MEAL_ALLERGENS = [
  { code: "gluten", letter: "A" },
  { code: "crustaceans", letter: "B" },
  { code: "eggs", letter: "C" },
  { code: "fish", letter: "D" },
  { code: "peanuts", letter: "E" },
  { code: "soybeans", letter: "F" },
  { code: "milk", letter: "G" },
  { code: "nuts", letter: "H" },
  { code: "celery", letter: "L" },
  { code: "mustard", letter: "M" },
  { code: "sesame", letter: "N" },
  { code: "sulphites", letter: "O" },
  { code: "lupin", letter: "P" },
  { code: "molluscs", letter: "R" },
] as const;

export type MealAllergen = (typeof MEAL_ALLERGENS)[number]["code"];

const ORDER = new Map<string, number>(
  MEAL_ALLERGENS.map((allergen, index) => [allergen.code, index]),
);

function isMealAllergen(code: string): code is MealAllergen {
  return ORDER.has(code);
}

/** Known codes once each, in legend order; unknown wire values are dropped. */
export function normalizeMealAllergens(
  codes: readonly string[] | null | undefined,
): MealAllergen[] {
  const known = new Set((codes ?? []).filter(isMealAllergen));
  return MEAL_ALLERGENS.map((allergen) => allergen.code).filter((code) =>
    known.has(code),
  );
}

export function mealAllergenLetter(code: MealAllergen): string {
  return MEAL_ALLERGENS[ORDER.get(code) ?? 0]!.letter;
}

/** The allergens used by any of the dishes, in legend order. */
export function usedMealAllergens(
  dishes: readonly { readonly allergens?: readonly MealAllergen[] }[],
): MealAllergen[] {
  return normalizeMealAllergens(dishes.flatMap((dish) => dish.allergens ?? []));
}
