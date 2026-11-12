import { describe, expect, it } from "vitest";
import type { HolidayFormField } from "~/lib/traeger-holiday-form-api";
import { questionsRepeatingHealthFields } from "./holiday-form-health";

const question = (
  key: string,
  label: string,
  type: HolidayFormField["type"] = "textarea",
): HolidayFormField => ({ key, label, type, sort_order: 0 });

describe("questionsRepeatingHealthFields (#6686)", () => {
  it("names own questions about allergies, intolerances or medication", () => {
    const fields = [
      question(
        "lebensmittelunvertraeglichkeiten",
        "Lebensmittelunverträglichkeiten",
      ),
      question("allergie", "Hat Ihr Kind eine Allergie?", "boolean"),
      question("medis", "Welche Medikamente nimmt Ihr Kind?"),
      question("tetanus", "Tetanus-Impfschutz", "select"),
      question("klasse", "Klasse", "text"),
      question("info", "Hinweis zu Medikamenten", "information"),
    ];
    expect(questionsRepeatingHealthFields(fields).map((f) => f.key)).toEqual([
      "lebensmittelunvertraeglichkeiten",
      "allergie",
      "medis",
    ]);
  });

  it("finds nothing in a form without such a question", () => {
    expect(
      questionsRepeatingHealthFields([question("klasse", "Klasse", "text")]),
    ).toEqual([]);
  });
});
