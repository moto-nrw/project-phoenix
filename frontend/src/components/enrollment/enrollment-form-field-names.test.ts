import { describe, expect, it } from "vitest";

import {
  fieldNameForServerPath,
  formFieldName,
} from "./enrollment-form-field-names";

// The backend names a rejected field by its JSON path; the form marks the
// input that renders under formFieldName. Both sides must name the same one.
describe("fieldNameForServerPath (#2515)", () => {
  it.each([
    ["guardian_email", formFieldName.guardian("email")],
    ["children.0.first_name", formFieldName.child(0, "first_name")],
    [
      "children.1.custom_data.allergies",
      formFieldName.childCustom(1, "allergies"),
    ],
    [
      "children.0.custom_data.student.departure_companion_note",
      formFieldName.childCompanionNote(0),
    ],
    ["custom_data.siblings", formFieldName.custom("siblings")],
    ["consent_flags.agb", formFieldName.consent("agb")],
    [
      "additional_guardians.1.email",
      formFieldName.additionalGuardian(1, "email"),
    ],
  ])("marks the input of %s", (path, inputName) => {
    expect(fieldNameForServerPath(path)).toBe(inputName);
  });
});
