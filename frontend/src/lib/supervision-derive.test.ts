import { describe, expect, it } from "vitest";
import {
  deriveSupervision,
  sameGroups,
  sameSupervision,
  sortNavigationGroups,
  type SchulhofStatus,
  type SupervisedGroupPayload,
} from "./supervision-derive";

const schulhof: SchulhofStatus = {
  exists: true,
  room_id: 9,
  room_name: "Schulhof",
  active_group_id: 42,
  is_user_supervising: false,
};

const room = (
  id: number,
  roomId: number,
  roomName: string,
  groupName?: string,
): SupervisedGroupPayload => ({
  id,
  group_id: id,
  room_id: roomId,
  room: { id: roomId, name: roomName },
  ...(groupName && { actual_group: { id: id * 10, name: groupName } }),
});

describe("deriveSupervision", () => {
  it("reports no supervision when the request failed and Schulhof is absent", () => {
    expect(deriveSupervision(null, null, true)).toEqual({
      isSupervising: false,
      supervisedRoomId: undefined,
      supervisedRoomName: undefined,
      supervisedRooms: [],
      overviewEnabled: false,
    });
  });

  it("keeps Schulhof alone supervisable, but never as an enabled overview", () => {
    const derived = deriveSupervision([], schulhof, true);
    expect(derived.isSupervising).toBe(true);
    expect(derived.supervisedRoomId).toBe("schulhof");
    expect(derived.supervisedRooms).toEqual([
      { id: "schulhof", name: "Schulhof", groupId: "42", isSchulhof: true },
    ]);
    expect(derived.overviewEnabled).toBe(true);
    expect(deriveSupervision(null, schulhof, true).overviewEnabled).toBe(false);
  });

  it("sorts rooms by German name, filters Schulhof, appends it last", () => {
    const derived = deriveSupervision(
      [room(1, 5, "Zebra"), room(2, 9, "Schulhof"), room(3, 6, "Äpfel")],
      schulhof,
      false,
    );
    expect(derived.supervisedRooms.map((r) => r.name)).toEqual([
      "Äpfel",
      "Zebra",
      "Schulhof",
    ]);
    expect(derived.supervisedRoomId).toBe("5");
    expect(derived.supervisedRoomName).toBe("Zebra");
    expect(derived.overviewEnabled).toBe(false);
  });

  it("suffixes the activity name when two sessions share a room (#2265)", () => {
    const derived = deriveSupervision(
      [room(1, 5, "Aula", "Chor"), room(2, 5, "Aula", "Theater")],
      null,
      true,
    );
    expect(derived.supervisedRooms.map((r) => r.name)).toEqual([
      "Chor · Aula",
      "Theater · Aula",
    ]);
    expect(derived.supervisedRooms.map((r) => r.groupId)).toEqual(["1", "2"]);
  });

  it("falls back to a generic room label without a room object", () => {
    const derived = deriveSupervision(
      [{ id: 1, group_id: 1, room_id: 7 }],
      null,
      false,
    );
    expect(derived.supervisedRoomName).toBe("Room 7");
    expect(derived.supervisedRooms).toEqual([]);
  });
});

describe("sameSupervision", () => {
  it("treats a changed active group in the same room as a change", () => {
    const a = deriveSupervision([room(1, 5, "Aula")], null, true);
    const b = deriveSupervision([room(2, 5, "Aula")], null, true);
    expect(sameSupervision(a, a)).toBe(true);
    expect(sameSupervision(a, b)).toBe(false);
  });

  it("treats a changed overview flag as a change", () => {
    const a = deriveSupervision([room(1, 5, "Aula")], null, true);
    const b = deriveSupervision([room(1, 5, "Aula")], null, false);
    expect(sameSupervision(a, b)).toBe(false);
  });
});

describe("groups", () => {
  it("sorts by German locale without mutating the input", () => {
    const input = [
      { id: "1", name: "Zebra" },
      { id: "2", name: "Äpfel" },
    ];
    expect(sortNavigationGroups(input).map((g) => g.name)).toEqual([
      "Äpfel",
      "Zebra",
    ]);
    expect(input[0]?.name).toBe("Zebra");
  });

  it("compares the fields the navigation renders", () => {
    const a = [{ id: "1", name: "A", is_personal: true }];
    expect(sameGroups(a, [{ id: "1", name: "A", is_personal: true }])).toBe(
      true,
    );
    expect(sameGroups(a, [{ id: "1", name: "A", is_personal: false }])).toBe(
      false,
    );
    expect(sameGroups(a, [])).toBe(false);
  });
});
