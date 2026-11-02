import { describe, expect, it } from "vitest";

import de from "~/i18n/messages/de.generated";
import en from "~/i18n/messages/en.generated";
import pl from "~/i18n/messages/pl.generated";
import ru from "~/i18n/messages/ru.generated";
import sq from "~/i18n/messages/sq.generated";
import tr from "~/i18n/messages/tr.generated";
import uk from "~/i18n/messages/uk.generated";

import {
  MEAL_ALLERGENS,
  mealAllergenLetter,
  normalizeMealAllergens,
  usedMealAllergens,
} from "./meal-allergens";

describe("meal allergens (#4777)", () => {
  it("lists the 14 main allergens with distinct letters", () => {
    expect(MEAL_ALLERGENS).toHaveLength(14);
    expect(new Set(MEAL_ALLERGENS.map((a) => a.letter)).size).toBe(14);
    expect(mealAllergenLetter("gluten")).toBe("A");
    expect(mealAllergenLetter("molluscs")).toBe("R");
  });

  it("keeps known codes once each in legend order", () => {
    expect(
      normalizeMealAllergens(["milk", "strawberry", "fish", "gluten", "fish"]),
    ).toEqual(["gluten", "fish", "milk"]);
    expect(normalizeMealAllergens(undefined)).toEqual([]);
    expect(normalizeMealAllergens(null)).toEqual([]);
  });

  it("collects the allergens of a week for its legend", () => {
    expect(
      usedMealAllergens([
        { allergens: ["milk"] },
        {},
        { allergens: ["gluten", "milk"] },
      ]),
    ).toEqual(["gluten", "milk"]);
  });

  it("names every allergen in every language", () => {
    for (const messages of [de, en, pl, ru, sq, tr, uk]) {
      const names = messages.mealAllergens.names as Record<string, string>;
      for (const { code } of MEAL_ALLERGENS) {
        expect(names[code]?.trim()).toBeTruthy();
      }
    }
  });
});
