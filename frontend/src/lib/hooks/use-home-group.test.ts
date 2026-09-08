import { describe, expect, it } from "vitest";

import type {
  OgsLiveViewData,
  OgsLiveWireStudent,
} from "~/lib/ogs-group-live-api";

import { deriveHomeGroup } from "./use-home-group";

function student(
  id: string,
  overrides: Partial<OgsLiveWireStudent> = {},
): OgsLiveWireStudent {
  return {
    id,
    first_name: `Kind${id}`,
    last_name: "Test",
    school_class: "2a",
    current_location: "Anwesend - OGS-Raum 1",
    sick: false,
    excused: false,
    class_trip: false,
    ...overrides,
  };
}

function liveData(overrides: Partial<OgsLiveViewData> = {}): OgsLiveViewData {
  return {
    groups: [
      {
        id: "5",
        name: "Sternengruppe",
        roomId: "1",
        roomName: "OGS-Raum 1",
        viaSubstitution: false,
        isPersonal: true,
      },
    ],
    groupId: "5",
    students: [],
    roomStatus: {},
    pickupTimes: new Map(),
    trackingIndicators: { labels: [], results: {} },
    transfers: [],
    ...overrides,
  };
}

describe("deriveHomeGroup (#2180)", () => {
  it("nimmt nur Abholungen, die noch kommen, von Kindern, die da sind", () => {
    const data = liveData({
      students: [
        student("1"),
        student("2"),
        student("3", { sick: true }),
        student("4", { actual_pickup_time: "13:05" }),
        student("5"),
      ],
      pickupTimes: new Map([
        [
          "1",
          {
            pickupTime: "15:30",
            isException: false,
            notes: undefined,
            dayNotes: [],
          },
        ],
        [
          "2",
          {
            pickupTime: "14:30",
            isException: true,
            notes: "Oma holt ab",
            dayNotes: [{ id: "n1", content: "Musikunterricht danach" }],
          },
        ],
        // Krank: die Abholzeit gilt heute nicht.
        [
          "3",
          {
            pickupTime: "14:00",
            isException: false,
            notes: undefined,
            dayNotes: [],
          },
        ],
        // Schon abgeholt.
        [
          "4",
          {
            pickupTime: "13:00",
            isException: false,
            notes: undefined,
            dayNotes: [],
          },
        ],
        // Vorbei, aber noch da: keine kommende Abholung mehr.
        [
          "5",
          {
            pickupTime: "12:00",
            isException: false,
            notes: undefined,
            dayNotes: [],
          },
        ],
      ]),
    });

    const snapshot = deriveHomeGroup(data, "13:10");

    expect(snapshot.pickups.map((p) => [p.student.id, p.time])).toEqual([
      ["2", "14:30"],
      ["1", "15:30"],
    ]);
    expect(snapshot.pickups[0]).toMatchObject({
      note: "Oma holt ab, Musikunterricht danach",
      isException: true,
    });
    expect(snapshot.nextPickup).toBe("14:30");
    expect(snapshot.present).toBe(4);
    expect(snapshot.total).toBe(5);
    expect(snapshot.away.map((s) => s.id)).toEqual(["3"]);
  });

  // Vor dem ersten Rendern im Browser ist die Uhr leer: dann gilt jede
  // Abholzeit als kommend, statt dass die Karte kurz leer steht.
  it("zeigt bei unbekannter Uhrzeit alle Abholungen", () => {
    const data = liveData({
      students: [student("1")],
      pickupTimes: new Map([
        [
          "1",
          {
            pickupTime: "08:00",
            isException: false,
            notes: undefined,
            dayNotes: [],
          },
        ],
      ]),
    });

    expect(deriveHomeGroup(data, "").pickups).toHaveLength(1);
  });

  it("zählt anwesende Kinder außerhalb des Gruppenraums", () => {
    const data = liveData({
      students: [
        student("1"),
        student("2", { current_location: "Schulhof" }),
        student("3", { current_location: "HOME" }),
      ],
      roomStatus: {
        "1": { in_group_room: true },
        "2": { in_group_room: false },
        "3": { in_group_room: false },
      },
    });

    const snapshot = deriveHomeGroup(data, "13:10");

    // Das Kind zuhause zählt nicht als „außerhalb": es ist gar nicht da.
    expect(snapshot.elsewhere).toBe(1);
    expect(snapshot.present).toBe(2);
  });

  it("findet, wer längst da sein sollte", () => {
    const data = liveData({
      students: [
        // Überfällig, mit Notiz der Eltern.
        student("1", {
          current_location: "HOME",
          arrival_time: "09:15",
          arrival_notes: "Arzttermin, kommt danach",
        }),
        // Wird erst später erwartet.
        student("2", { current_location: "HOME", arrival_time: "14:00" }),
        // Krank: wird nicht erwartet.
        student("3", {
          current_location: "HOME",
          arrival_time: "08:00",
          sick: true,
        }),
        // Abgemeldet: wird nicht erwartet.
        student("4", {
          current_location: "HOME",
          arrival_time: "08:00",
          day_planning_status: "not_coming_today",
        }),
        // Schon da.
        student("5", { arrival_time: "08:00", actual_arrival_time: "08:02" }),
      ],
    });

    const snapshot = deriveHomeGroup(data, "13:10");

    expect(snapshot.missing).toEqual([
      {
        student: data.students[0],
        expected: "09:15",
        note: "Arzttermin, kommt danach",
      },
    ]);
    expect(snapshot.away.map((s) => s.id)).toEqual(["1", "2", "3", "4"]);
    // Ohne Uhr fehlt niemand: kein Alarm, der gleich wieder verschwindet.
    expect(deriveHomeGroup(data, "").missing).toEqual([]);
  });

  it("gibt ohne eigene Gruppe den leeren Stand zurück", () => {
    expect(deriveHomeGroup(liveData({ groupId: null }), "13:10").group).toBe(
      null,
    );
  });
});
