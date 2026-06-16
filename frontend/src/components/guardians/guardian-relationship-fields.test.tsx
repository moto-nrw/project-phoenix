import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  GuardianRoleSelect,
  RelationshipTypeSelect,
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

    fireEvent.change(screen.getByLabelText("Portalrolle"), {
      target: { value: "pickup_only" },
    });

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

    fireEvent.change(screen.getByLabelText("Beziehung zum Kind"), {
      target: { value: "relative" },
    });

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
