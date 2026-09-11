import { describe, expect, it } from "vitest";
import type {
  OpenRoomPayload,
  SupervisedGroupPayload,
} from "./supervision-derive";
import { deriveSupervision, sameSupervision } from "./supervision-derive";

// The navigation contract for released rooms (#3065). It replaces the previous
// synthetic "schulhof" tab, which could only ever be one room, only appeared
// when a session existed, and counted as the caller's own supervision.

const supervision = (
  id: number,
  roomId: number,
  roomName: string,
  activity?: string,
): SupervisedGroupPayload => ({
  id,
  room_id: roomId,
  group_id: id,
  room: { id: roomId, name: roomName },
  ...(activity ? { actual_group: { id, name: activity } } : {}),
});

const openRoom = (id: number, name: string): OpenRoomPayload => ({ id, name });

describe("deriveSupervision with released rooms", () => {
  it("lists several released rooms at once, by their real room id", () => {
    const result = deriveSupervision(
      null,
      [openRoom(7, "Turnhalle"), openRoom(3, "Werkraum")],
      false,
    );

    expect(result.supervisedRooms.map((room) => room.id)).toEqual(["7", "3"]);
    expect(result.supervisedRooms.every((room) => room.isOpenRoom)).toBe(true);
    expect(result.supervisedRooms.map((room) => room.groupId)).toEqual([
      "",
      "",
    ]);
  });

  it("shows a released room that has nothing running in it", () => {
    // The previous derivation took its rooms from running supervisions, so an
    // empty room could not appear at all.
    const result = deriveSupervision([], [openRoom(7, "Turnhalle")], true);

    expect(result.supervisedRooms).toHaveLength(1);
    expect(result.supervisedRooms[0]?.name).toBe("Turnhalle");
  });

  it("does not turn visibility into the caller's own supervision", () => {
    const result = deriveSupervision(null, [openRoom(7, "Turnhalle")], false);

    expect(result.isSupervising).toBe(false);
    expect(result.supervisedRoomId).toBeUndefined();
    expect(result.supervisedRoomName).toBeUndefined();
  });

  it("keeps own supervisions first and shared rooms after them", () => {
    const result = deriveSupervision(
      [supervision(1, 5, "Gruppenraum")],
      [openRoom(7, "Turnhalle")],
      true,
    );

    expect(result.isSupervising).toBe(true);
    expect(result.supervisedRooms.map((room) => room.name)).toEqual([
      "Gruppenraum",
      "Turnhalle",
    ]);
    expect(result.supervisedRooms[0]?.isOpenRoom).toBeUndefined();
    expect(result.supervisedRooms[1]?.isOpenRoom).toBe(true);
  });

  it("lists a released room once even when the caller supervises there", () => {
    // Otherwise the same place appears twice: once as an own supervision and
    // once as a shared room.
    const result = deriveSupervision(
      [supervision(1, 7, "Turnhalle")],
      [openRoom(7, "Turnhalle")],
      true,
    );

    const turnhalle = result.supervisedRooms.filter((room) => room.id === "7");
    expect(turnhalle).toHaveLength(1);
    expect(turnhalle[0]?.isOpenRoom).toBe(true);
    expect(turnhalle[0]?.sessionIds).toEqual(["1"]);
  });

  it("lists a released room once even with several sessions running there", () => {
    // Parallel sessions used to produce one navigation entry each, suffixed
    // with the activity name.
    const result = deriveSupervision(
      [
        supervision(1, 7, "Turnhalle", "Fußball"),
        supervision(2, 7, "Turnhalle", "Tanzen"),
      ],
      [openRoom(7, "Turnhalle")],
      true,
    );

    expect(
      result.supervisedRooms.filter((room) => room.id === "7"),
    ).toHaveLength(1);
    expect(result.supervisedRooms[0]?.sessionIds).toEqual(["1", "2"]);
  });

  it("still distinguishes parallel sessions in a room that is NOT released", () => {
    // The disambiguation stays where it is still needed.
    const result = deriveSupervision(
      [
        supervision(1, 5, "Gruppenraum", "Fußball"),
        supervision(2, 5, "Gruppenraum", "Tanzen"),
      ],
      [],
      true,
    );

    expect(result.supervisedRooms.map((room) => room.name)).toEqual([
      "Fußball · Gruppenraum",
      "Tanzen · Gruppenraum",
    ]);
  });

  it("treats a failed room load as unknown, not as 'no open rooms'", () => {
    const failed = deriveSupervision(
      [supervision(1, 5, "Gruppenraum")],
      null,
      true,
    );
    const empty = deriveSupervision(
      [supervision(1, 5, "Gruppenraum")],
      [],
      true,
    );

    expect(failed.supervisedRooms).toHaveLength(1);
    expect(empty.supervisedRooms).toHaveLength(1);
    // Both render the same navigation here; the distinction that matters is
    // that a null never fabricates an entry, and never removes the caller's
    // own supervision either.
    expect(failed.isSupervising).toBe(true);
  });

  it("orders released rooms by name", () => {
    const result = deriveSupervision(
      null,
      [
        openRoom(1, "Werkraum"),
        openRoom(2, "Ästhetikraum"),
        openRoom(3, "Turnhalle"),
      ],
      false,
    );

    expect(result.supervisedRooms.map((room) => room.name)).toEqual([
      "Ästhetikraum",
      "Turnhalle",
      "Werkraum",
    ]);
  });

  // „Aufsicht fortsetzen" auf der Startseite darf nur erscheinen, wenn die
  // Person selbst gerade Aufsicht führt — nicht, weil es einen offenen Raum
  // gibt, den sie betreten könnte (#2180).
  it("unterscheidet die eigene Aufsicht von der bloßen Möglichkeit dazu", () => {
    // Ein offener Raum steht bereit, die Person führt dort nichts: nichts
    // fortzusetzen.
    expect(
      deriveSupervision([], [openRoom(7, "Turnhalle")], true).ownSupervision,
    ).toBe(false);
    // Eigener Raum aus /api/me/groups/supervised (keine Übersicht).
    expect(
      deriveSupervision([supervision(1, 5, "Zebra")], [], false).ownSupervision,
    ).toBe(true);
    // Dieselbe Zeile aus der schulweiten Übersicht kann fremd sein; erst die
    // zusätzliche eigene Abfrage entscheidet.
    expect(
      deriveSupervision([supervision(1, 5, "Zebra")], [], true).ownSupervision,
    ).toBe(false);
    expect(
      deriveSupervision([supervision(1, 5, "Zebra")], [], true, [
        supervision(1, 5, "Zebra"),
      ]).ownSupervision,
    ).toBe(true);
  });

  it("counts an own supervision inside a released room as own supervision", () => {
    // Der offene Raum ersetzt den Schulhof: wer sich dort angeschlossen hat,
    // führt Aufsicht und muss sie fortsetzen können.
    expect(
      deriveSupervision(
        [supervision(1, 7, "Turnhalle")],
        [openRoom(7, "Turnhalle")],
        false,
      ).ownSupervision,
    ).toBe(true);
    expect(
      deriveSupervision(
        [supervision(1, 7, "Turnhalle")],
        [openRoom(7, "Turnhalle")],
        true,
        [supervision(1, 7, "Turnhalle")],
      ).ownSupervision,
    ).toBe(true);
  });

  it("reports overviewEnabled only when the overview endpoint answered", () => {
    expect(deriveSupervision([], [], true).overviewEnabled).toBe(true);
    expect(deriveSupervision([], [], false).overviewEnabled).toBe(false);
    expect(deriveSupervision(null, [], true).overviewEnabled).toBe(false);
  });
});

describe("sameSupervision", () => {
  it("treats an added released room as a change", () => {
    const before = deriveSupervision([], [], true);
    const after = deriveSupervision([], [openRoom(7, "Turnhalle")], true);

    expect(sameSupervision(before, after)).toBe(false);
  });

  it("treats an unchanged room list as unchanged", () => {
    const before = deriveSupervision([], [openRoom(7, "Turnhalle")], true);
    const after = deriveSupervision([], [openRoom(7, "Turnhalle")], true);

    expect(sameSupervision(before, after)).toBe(true);
  });
});
