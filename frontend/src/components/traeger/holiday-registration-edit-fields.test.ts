import { describe, expect, it } from "vitest";
import type { HolidayFormField } from "~/lib/holiday-form-field";
import {
  editableQuestions,
  editedFieldsText,
} from "./holiday-registration-edit-fields";

const FIELDS: HolidayFormField[] = [
  {
    key: "tetanus_impfschutz",
    label: "Tetanus-Impfschutz",
    type: "select",
    applies_to_child: true,
    health: true,
    sort_order: 0,
    options: [{ label: "Ja", value: "ja" }],
  },
  {
    key: "tetanus_datum",
    label: "Letzte Tetanus-Impfung",
    type: "select",
    applies_to_child: true,
    required: true,
    sort_order: 1,
    options: [{ label: "2024", value: "2024" }],
    visible_when: {
      source: "field",
      field: "tetanus_impfschutz",
      operator: "eq",
      value: "ja",
    },
  },
  {
    key: "tetanus_nachweis",
    label: "Nachweis liegt vor",
    type: "boolean",
    applies_to_child: true,
    required: true,
    sort_order: 2,
    visible_when: {
      source: "field",
      field: "tetanus_datum",
      operator: "eq",
      value: "2024",
    },
  },
  {
    key: "darf_allein",
    label: "Darf allein nach Hause",
    type: "boolean",
    applies_to_child: true,
    sort_order: 3,
  },
  {
    key: "allein_ab",
    label: "Ab wann allein",
    type: "text",
    applies_to_child: true,
    required: true,
    sort_order: 4,
    visible_when: {
      source: "field",
      field: "darf_allein",
      operator: "eq",
      value: true,
    },
  },
];

describe("editableQuestions", () => {
  it("shows questions behind a health question without their condition and as optional (#5629)", () => {
    const questions = editableQuestions(FIELDS, true);
    expect(questions.map((field) => field.key)).toEqual([
      "tetanus_datum",
      "tetanus_nachweis",
      "darf_allein",
      "allein_ab",
    ]);
    const byKey = new Map(questions.map((field) => [field.key, field]));
    // Directly and through another question behind the sealed answer.
    for (const key of ["tetanus_datum", "tetanus_nachweis"]) {
      expect(byKey.get(key)?.visible_when).toBeNull();
      expect(byKey.get(key)?.required).toBe(false);
    }
    // A condition the Träger-Büro can evaluate stays as the form has it.
    expect(byKey.get("allein_ab")?.visible_when?.field).toBe("darf_allein");
    expect(byKey.get("allein_ab")?.required).toBe(true);
  });

  it("names the address in the history, never its value", () => {
    expect(editedFieldsText(["guardian_address"], FIELDS)).toBe("Anschrift");
    expect(
      editedFieldsText(["children.0.custom_data.tetanus_datum"], FIELDS),
    ).toBe("Letzte Tetanus-Impfung von Kind 1");
  });
});
