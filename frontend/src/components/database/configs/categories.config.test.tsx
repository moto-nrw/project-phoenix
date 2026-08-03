/**
 * Tests for the activity category Stammdaten configuration (#2131).
 */
import { describe, it, expect } from "vitest";
import {
  categoriesConfig,
  CATEGORY_NAME_MAX_LENGTH,
} from "./categories.config";
import { prepareCategoryForBackend } from "@/lib/activity-helpers";

describe("categoriesConfig", () => {
  it("exports a valid entity config", () => {
    expect(categoriesConfig.name).toEqual({
      singular: "Kategorie",
      plural: "Kategorien",
    });
    expect(categoriesConfig.api.basePath).toBe("/api/activities/categories");
  });

  it("opts into archived categories so they can be restored", () => {
    expect(categoriesConfig.api.listParams).toEqual({
      include_archived: "true",
    });
  });

  it("does not request system categories", () => {
    // Schulhof/WC are backend infrastructure; every write on them is refused,
    // so listing them would only offer dead-end rows.
    expect(categoriesConfig.api.listParams).not.toHaveProperty(
      "include_system",
    );
  });

  it("exposes name, description and color fields", () => {
    const fields = categoriesConfig.form.sections[0]?.fields ?? [];
    expect(fields.map((field) => field.name)).toEqual([
      "name",
      "description",
      "color",
    ]);
  });

  it("requires a non-empty name within the backend length limit", () => {
    const nameField = categoriesConfig.form.sections[0]?.fields.find(
      (field) => field.name === "name",
    );
    expect(nameField?.required).toBe(true);
    expect(nameField?.validation?.("   ")).toBe("Name ist erforderlich");
    expect(nameField?.validation?.("x".repeat(CATEGORY_NAME_MAX_LENGTH))).toBe(
      null,
    );
    expect(
      nameField?.validation?.("x".repeat(CATEGORY_NAME_MAX_LENGTH + 1)),
    ).toContain("höchstens");
  });

  it("trims the name before submitting", () => {
    const transformed = categoriesConfig.form.transformBeforeSubmit?.({
      name: "  Essen  ",
    });
    expect(transformed?.name).toBe("Essen");
  });
});

describe("prepareCategoryForBackend", () => {
  it("forwards only the editable fields", () => {
    expect(
      prepareCategoryForBackend({
        id: "7",
        name: "  Essen ",
        description: " Mittagessen ",
        color: "#FF9500",
        isSystem: true,
        archivedAt: "2026-08-01T10:00:00Z",
        usageCount: 3,
      }),
    ).toEqual({
      name: "Essen",
      description: "Mittagessen",
      color: "#FF9500",
    });
  });
});
