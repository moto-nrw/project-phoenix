import { createTranslator } from "next-intl";
import { describe, expect, it } from "vitest";

import de from "./messages/de.json";
import en from "./messages/en.json";
import pl from "./messages/pl.json";
import ru from "./messages/ru.json";
import sq from "./messages/sq.json";
import tr from "./messages/tr.json";
import uk from "./messages/uk.json";

// #3164: the lead days of the meal cutoff are ICU plurals in every locale.
const catalogs = { de, en, pl, ru, sq, tr, uk };
const leadKeys = [
  ["mealParticipantList", "cutoffLead"],
  ["mealParticipantList", "weekCutoff"],
  ["parentMealPlan", "participationCutoffLead"],
  ["parentMealPlan", "participationSicknessLead"],
] as const;

describe("meal cutoff lead-day texts", () => {
  it.each(Object.entries(catalogs))(
    "formats every lead-day count in %s",
    (locale, messages) => {
      for (const [namespace, key] of leadKeys) {
        const t = createTranslator({ locale, messages, namespace });
        for (const days of [1, 2, 5, 10]) {
          const text = t(key, { time: "09:00", days });
          expect(text).toContain("09:00");
          expect(text).not.toMatch(/[{}]/);
        }
      }
    },
  );

  it("reads naturally in German", () => {
    const staff = createTranslator({
      locale: "de",
      messages: de,
      namespace: "mealParticipantList",
    });
    const parents = createTranslator({
      locale: "de",
      messages: de,
      namespace: "parentMealPlan",
    });
    expect(staff("weekCutoff", { time: "09:00", days: 0 })).toBe(
      "Änderungen sind bis 09:00 Uhr am Essenstag möglich.",
    );
    expect(staff("weekCutoff", { time: "09:00", days: 1 })).toBe(
      "Änderungen sind bis 09:00 Uhr am Werktag davor möglich.",
    );
    expect(staff("cutoffLead", { time: "14:00", days: 3 })).toBe(
      "Änderungen für diesen Tag sind bis 14:00 Uhr 3 Werktage vorher möglich.",
    );
    expect(parents("participationCutoffLead", { time: "14:00", days: 1 })).toBe(
      "Sie können die Anmeldung bis 14:00 Uhr am Werktag vor dem Essen ändern.",
    );
  });
});
