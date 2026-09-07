import { describe, expect, it } from "vitest";
import { buildRoomFormSections } from "./room-form-sections";
import type { Room } from "@/lib/room-helpers";

// Which rooms may be released as an "offener Raum" (#3064). The switch exists
// for ordinary rooms and for the Schulhof, and is absent for the toilets —
// where the backend refuses the release outright, so a switch there could only
// ever fail.

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

function fieldNames(room: Room | null | undefined): string[] {
  return buildRoomFormSections(room).flatMap((section) =>
    section.fields.map((field) => field.name),
  );
}

describe("buildRoomFormSections and the room release", () => {
  it("offers the release switch for an ordinary room", () => {
    expect(fieldNames(baseRoom)).toContain("isOpenRoom");
  });

  it("offers the release switch for the Schulhof so it can be switched off", () => {
    expect(fieldNames({ ...baseRoom, name: "Schulhof" })).toContain(
      "isOpenRoom",
    );
  });

  it("drops the release switch for the WC", () => {
    expect(fieldNames({ ...baseRoom, name: "WC" })).not.toContain("isOpenRoom");
  });

  it("drops the release switch for the Toilette alias", () => {
    expect(fieldNames({ ...baseRoom, name: "Toilette" })).not.toContain(
      "isOpenRoom",
    );
  });

  it("offers the release switch when creating a new room", () => {
    // A new room has no entity yet; the switch has to be reachable so an
    // administrator can release a room right away.
    expect(fieldNames(null)).toContain("isOpenRoom");
    expect(fieldNames(undefined)).toContain("isOpenRoom");
  });

  it("keeps the toilet rooms' other restrictions intact", () => {
    const names = fieldNames({ ...baseRoom, name: "WC" });
    expect(names).not.toContain("color");
    expect(names).toContain("name");
  });

  it("explains what the release does and does not mean", () => {
    // The likeliest wrong reading of "Freigabe" is "I have taken supervision
    // here", so the hint has to rule that out at the switch itself.
    const field = buildRoomFormSections(baseRoom)
      .flatMap((section) => section.fields)
      .find((entry) => entry.name === "isOpenRoom");

    expect(field?.label).toBe("Offener Raum");
    expect(field?.helperText).toContain("Ziel");
    expect(field?.helperText).toContain("Aufsicht");
  });
});
