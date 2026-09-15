import { describe, expect, it } from "vitest";

import de from "./messages/de.json";
import en from "./messages/en.json";
import pl from "./messages/pl.json";
import ru from "./messages/ru.json";
import sq from "./messages/sq.json";
import tr from "./messages/tr.json";
import uk from "./messages/uk.json";

const invariantValues = new Set(["PDF", "Excel"]);

describe("meal participant list translations", () => {
  it.each([
    ["en", en.mealParticipantList],
    ["ru", ru.mealParticipantList],
    ["sq", sq.mealParticipantList],
    ["pl", pl.mealParticipantList],
    ["tr", tr.mealParticipantList],
    ["uk", uk.mealParticipantList],
  ])("has every key translated in %s", (_locale, catalog) => {
    expect(Object.keys(catalog).sort()).toEqual(
      Object.keys(de.mealParticipantList).sort(),
    );

    const untranslated = Object.entries(catalog).filter(
      ([key, value]) =>
        value === de.mealParticipantList[key as keyof typeof catalog] &&
        !invariantValues.has(value),
    );
    expect(untranslated).toEqual([]);
  });
});
