import { describe, expect, it } from "vitest";
import { buildActivityFormSections } from "./activity-form-sections";
import type { Activity } from "@/lib/activity-helpers";

const baseActivity: Activity = {
  id: "1",
  name: "Mathe AG",
  max_participant: 12,
  is_open_ags: false,
  supervisor_id: "1",
  ag_category_id: "1",
  category_name: "Sport",
  created_at: new Date(),
  updated_at: new Date(),
};

describe("buildActivityFormSections", () => {
  it("leaves the name field editable for regular activities", () => {
    const sections = buildActivityFormSections(baseActivity);
    const nameField = sections
      .flatMap((section) => section.fields)
      .find((field) => field.name === "name");

    expect(nameField?.disabled).toBeFalsy();
  });

  it("disables the name field for system activities", () => {
    const sections = buildActivityFormSections({
      ...baseActivity,
      name: "Schulhof Freispiel",
    });
    const nameField = sections
      .flatMap((section) => section.fields)
      .find((field) => field.name === "name");

    expect(nameField?.disabled).toBe(true);
    expect(nameField?.helperText).toBe(
      "Systemaktivität: Name kann nicht geändert werden",
    );
  });
});
