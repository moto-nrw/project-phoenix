import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { type ComponentProps } from "react";
import { CatalogColorField } from "@/components/ui/database/catalog-color-field";
import { LOCATION_COLORS } from "@/lib/location-helper";
import { buildRoomFormSections } from "./room-form-sections";
import type { Room } from "@/lib/room-helpers";

// The category options come from the school's room categories (#4352).
vi.mock("@/lib/room-category-api", () => ({
  roomCategoryOptions: vi.fn(() =>
    Promise.resolve([
      { value: "Gruppenraum", label: "Gruppenraum" },
      { value: "Sport", label: "Sport" },
    ]),
  ),
}));

async function categoryOptions(room: Room | null) {
  const field = buildRoomFormSections(room)
    .flatMap((section) => section.fields)
    .find((candidate) => candidate.name === "category");
  const options = field?.options;
  return typeof options === "function" ? options() : options;
}

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
  it("offers the school's room categories and links to their list", async () => {
    const categoryField = buildRoomFormSections(baseRoom)
      .flatMap((section) => section.fields)
      .find((field) => field.name === "category");

    expect(await categoryOptions(baseRoom)).toEqual([
      { value: "Gruppenraum", label: "Gruppenraum" },
      { value: "Sport", label: "Sport" },
    ]);
    expect(categoryField?.manageLink).toEqual({
      href: "/database/room-categories",
      label: "Raumkategorien verwalten",
    });
  });

  it("keeps a category the list no longer has selectable", async () => {
    expect(
      await categoryOptions({ ...baseRoom, category: "Legacy-Kategorie" }),
    ).toContainEqual({
      value: "Legacy-Kategorie",
      label: "Legacy-Kategorie (nicht in den Raumkategorien)",
    });
  });

  it("keeps the system category of the Schulhof without a remark", async () => {
    expect(
      await categoryOptions({
        ...baseRoom,
        name: "Schulhof",
        category: "Schulhof",
      }),
    ).toContainEqual({ value: "Schulhof", label: "Schulhof" });
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
    expect(colorField?.component).toBe(CatalogColorField);

    // With no colour set the swatch must show the orange the yard badge
    // actually renders, not the generic room blue, and the hint has to say so.
    const preset = colorField?.componentProps as ComponentProps<
      typeof CatalogColorField
    >;
    render(
      <CatalogColorField
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

  it("previews the category color for a room without its own", () => {
    // Only the Schulhof gets the orange-default binding; every other room
    // previews the color its badge falls back to: its category's (#4352),
    // else the picker's generic default.
    const colorProps = (room: Room) =>
      buildRoomFormSections(room)
        .flatMap((section) => section.fields)
        .find((field) => field.name === "color")?.componentProps;

    const withCategory = colorProps({
      ...baseRoom,
      name: "Werkraum",
      categoryColor: "#283590",
    });
    expect(withCategory?.defaultHex).toBe("#283590");
    expect(withCategory?.hint).toMatch(/Farbe seiner Raumkategorie/);

    const withoutCategory = colorProps({ ...baseRoom, name: "Werkraum" });
    expect(withoutCategory?.defaultHex).toBeUndefined();
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
