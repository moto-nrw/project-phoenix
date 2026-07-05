import { describe, expect, it } from "vitest";

import type { FormField } from "./enrollment-form-schema-api";
import {
  getDepartureSchemaHealth,
  isLegacyDepartureField,
  isModernDepartureField,
  legacyDepartureTargetLabel,
} from "./enrollment-departure-schema-health";

function field(patch: Partial<FormField>): FormField {
  return {
    key: patch.target?.replace(/\./g, "_") ?? "custom",
    label: "Feld",
    type: "text",
    sort_order: 0,
    applies_to_child: true,
    ...patch,
  };
}

describe("enrollment departure schema health", () => {
  it("detects the modern allowed departure modes field", () => {
    const modern = field({
      target: "student.allowed_departure_modes",
      type: "weekday_multi_mode",
    });

    expect(isModernDepartureField(modern)).toBe(true);
    expect(getDepartureSchemaHealth([modern])).toMatchObject({
      hasModernDepartureModes: true,
      needsLegacyCleanup: false,
    });
  });

  it("detects every legacy departure target", () => {
    const legacy = [
      field({ target: "student.departure", type: "weekday_mode" }),
      field({ target: "student.bus_days", type: "weekday_boolean" }),
      field({ target: "student.bus", type: "weekday_boolean" }),
      field({ target: "student.pickup_status", type: "weekday_boolean" }),
    ];

    const health = getDepartureSchemaHealth(legacy);

    expect(legacy.every(isLegacyDepartureField)).toBe(true);
    expect(health.needsLegacyCleanup).toBe(true);
    expect(health.legacyFields).toHaveLength(4);
    expect(health.legacyLabels).toEqual([
      "Geh- und Abholregelung",
      "Buskind",
      "Abholregelung",
    ]);
  });

  it("does not warn for unrelated fields", () => {
    const health = getDepartureSchemaHealth([
      field({ target: "student.health_info", type: "textarea" }),
      field({ target: "", type: "text", applies_to_child: false }),
    ]);

    expect(health).toMatchObject({
      hasModernDepartureModes: false,
      legacyFields: [],
      legacyLabels: [],
      needsLegacyCleanup: false,
    });
  });

  it("labels unknown or non-legacy targets as null", () => {
    expect(legacyDepartureTargetLabel("student.health_info")).toBeNull();
    expect(legacyDepartureTargetLabel(undefined)).toBeNull();
    expect(legacyDepartureTargetLabel("student.pickup_status")).toBe(
      "Abholregelung",
    );
  });
});
