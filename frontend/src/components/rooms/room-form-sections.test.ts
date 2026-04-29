import { describe, expect, it } from "vitest";
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
});
