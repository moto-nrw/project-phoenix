import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { type ComponentProps } from "react";
import { RoomColorField } from "@/components/ui/database/room-color-field";
import { LOCATION_COLORS } from "@/lib/location-helper";
import { buildRoomFormSections } from "./room-form-sections";
import type { Room } from "@/lib/room-helpers";

const baseRoom: Room = {
  id: "1",
  name: "Test Raum",
  category: "Gruppenraum",
  building: "Altbau",
  floor: 1,
  capacity: 20,
  color: "#4F46E5",
  isOccupied: false,
};

describe("buildRoomFormSections", () => {
  it("keeps standard room categories unchanged", () => {
    const sections = buildRoomFormSections(baseRoom);
    const categoryField = sections
      .flatMap((section) => section.fields)
      .find((field) => field.name === "category");

    expect(Array.isArray(categoryField?.options)).toBe(true);
    expect(categoryField?.options).toContainEqual({
      value: "Gruppenraum",
      label: "Gruppenraum",
    });
  });

  it("adds the active legacy category to the selectable options", () => {
    const sections = buildRoomFormSections({
      ...baseRoom,
      category: "Legacy-Kategorie",
    });
    const categoryField = sections
      .flatMap((section) => section.fields)
      .find((field) => field.name === "category");

    expect(categoryField?.options).toContainEqual({
      value: "Legacy-Kategorie",
      label: "Legacy-Kategorie (Legacy)",
    });
  });

  it("disables the name field for system rooms", () => {
    const sections = buildRoomFormSections({
      ...baseRoom,
      name: "Schulhof",
    });
    const nameField = sections
      .flatMap((section) => section.fields)
      .find((field) => field.name === "name");

    expect(nameField?.disabled).toBe(true);
    expect(nameField?.helperText).toBe(
      "Systemraum: Name kann nicht geändert werden",
    );
  });

  // ===========================================================================
  // Issue #1324 — system rooms must NOT show the color picker. Backend
  // rejects color changes with a 403, so leaving the picker visible would let
  // admins pick a color, hit save, and see a confusing German error toast for
  // what looks like a normal field. This filter is a UX guard, not a security
  // one — the backend stays the source of truth.
  //
  // Issue #2405 narrowed that rule to the toilet rooms: the Schulhof's color
  // IS admin-configurable now (schools color-code rooms and tablets), so its
  // picker must be present while name/delete protection stays.
  // ===========================================================================
  it("keeps the color field for the Schulhof and binds its orange default", () => {
    const sections = buildRoomFormSections({
      ...baseRoom,
      name: "Schulhof",
    });
    const colorField = sections
      .flatMap((section) => section.fields)
      .find((field) => field.name === "color");

    expect(colorField).toBeDefined();
    // Same shared kit component as every other room; only its preset props
    // differ, bound through the field config.
    expect(colorField?.component).toBe(RoomColorField);

    // With no colour set the swatch must show the orange the yard badge
    // actually renders, not the generic room blue, and the hint has to say so.
    const preset = colorField?.componentProps as ComponentProps<
      typeof RoomColorField
    >;
    render(
      <RoomColorField
        {...preset}
        value={null}
        onChange={vi.fn()}
        label="Farbe"
      />,
    );

    expect(screen.getByLabelText("Farbe")).toHaveValue(
      LOCATION_COLORS.SCHOOLYARD.toLowerCase(),
    );
    expect(
      screen.getByText(/Ohne eigene Farbe erscheint der Schulhof in Orange/),
    ).toBeInTheDocument();
  });

  it("strips the color field for system rooms (WC)", () => {
    const sections = buildRoomFormSections({
      ...baseRoom,
      name: "WC",
    });
    const fieldNames = sections.flatMap((section) =>
      section.fields.map((f) => f.name),
    );
    expect(fieldNames).not.toContain("color");
  });

  it("strips the color field for system rooms (Toilette)", () => {
    const sections = buildRoomFormSections({
      ...baseRoom,
      name: "Toilette",
    });
    const fieldNames = sections.flatMap((section) =>
      section.fields.map((f) => f.name),
    );
    expect(fieldNames).not.toContain("color");
  });

  it("uses the plain color picker for regular rooms", () => {
    // Only the Schulhof gets the orange-default binding; every other room
    // keeps the unbound picker with its blue fallback preview.
    const sections = buildRoomFormSections({ ...baseRoom, name: "Werkraum" });
    const colorField = sections
      .flatMap((section) => section.fields)
      .find((field) => field.name === "color");

    expect(colorField?.component).toBe(RoomColorField);
    expect(colorField?.componentProps).toBeUndefined();
  });

  it("keeps the color field for regular rooms", () => {
    // Regression guard: the filter must trigger ONLY for system rooms.
    // If someone refactors and accidentally drops the isSystemRoom guard,
    // every room loses its color picker silently.
    const sections = buildRoomFormSections({
      ...baseRoom,
      name: "Werkraum",
    });
    const fieldNames = sections.flatMap((section) =>
      section.fields.map((f) => f.name),
    );
    expect(fieldNames).toContain("color");
  });

  it("returns the color field unchanged when given null/undefined room", () => {
    // The form is also rendered when creating a new room (no Room object
    // exists yet). isSystemRoom(null) returns false, so the color field
    // must be present — otherwise admins can never set a color on creation.
    const sections = buildRoomFormSections(null);
    const fieldNames = sections.flatMap((section) =>
      section.fields.map((f) => f.name),
    );
    expect(fieldNames).toContain("color");
  });
});
