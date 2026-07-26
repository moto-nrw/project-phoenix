import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  GuardianRoleSelect,
  RelationshipTypeSelect,
  defaultGuardianRoleForRelationshipType,
  guardianRoleOperationalDefaults,
} from "./guardian-relationship-fields";

describe("GuardianRoleSelect", () => {
  it("renders role options and emits the selected role", () => {
    const onChange = vi.fn();

    render(
      <GuardianRoleSelect
        id="role"
        value="legal_guardian"
        onChange={onChange}
      />,
    );

    fireEvent.click(screen.getByLabelText("Portalrolle"));
    fireEvent.click(screen.getByRole("option", { name: "Nur Abholung" }));

    expect(onChange).toHaveBeenCalledWith("pickup_only");
  });
});

describe("RelationshipTypeSelect", () => {
  it("renders relationship options and emits the selected type", () => {
    const onChange = vi.fn();

    render(
      <RelationshipTypeSelect
        id="relationship"
        value="parent"
        onChange={onChange}
      />,
    );

    fireEvent.click(screen.getByLabelText("Beziehung zum Kind"));
    fireEvent.click(screen.getByRole("option", { name: "Verwandte/r" }));

    expect(onChange).toHaveBeenCalledWith("relative");
  });
});

describe("guardianRoleOperationalDefaults", () => {
  it.each([
    [
      "primary_guardian",
      { isPrimary: true, canPickup: false, isEmergencyContact: false },
    ],
    [
      "legal_guardian",
      { isPrimary: false, canPickup: false, isEmergencyContact: false },
    ],
    [
      "co_guardian",
      { isPrimary: false, canPickup: false, isEmergencyContact: false },
    ],
    [
      "emergency_contact",
      { isPrimary: false, canPickup: false, isEmergencyContact: true },
    ],
    [
      "pickup_only",
      { isPrimary: false, canPickup: true, isEmergencyContact: false },
    ],
    [
      "social_worker",
      { isPrimary: false, canPickup: false, isEmergencyContact: false },
    ],
    ["custom", {}],
  ] as const)("returns defaults for %s", (role, expected) => {
    expect(guardianRoleOperationalDefaults(role)).toEqual(expected);
  });
});

describe("defaultGuardianRoleForRelationshipType", () => {
  it.each([
    ["parent", "legal_guardian"],
    ["guardian", "legal_guardian"],
    ["relative", "custom"],
    ["other", "custom"],
  ] as const)("maps %s to %s", (relationshipType, expected) => {
    expect(defaultGuardianRoleForRelationshipType(relationshipType)).toBe(
      expected,
    );
  });
});
