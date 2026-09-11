import { describe, expect, it } from "vitest";

import type { OwnAssignment } from "~/lib/shift-helpers";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

import {
  deriveHomeNow,
  deriveOwnNow,
  deriveSchoolNow,
  nowActions,
} from "./home-now";

function assignment(overrides: Partial<OwnAssignment> = {}): OwnAssignment {
  return {
    instanceId: "1",
    date: "2026-09-08",
    startTime: "10:00",
    endTime: "11:00",
    title: "Lernzeit",
    groupName: "Mondgruppe",
    roomName: "OGS-Raum 1",
    status: "planned",
    cancelled: false,
    isPrimary: true,
    isAbsent: false,
    isSubstitute: false,
    absenceReason: null,
    cancelReason: null,
    understaffedAck: false,
    ...overrides,
  };
}

function block(
  overrides: Partial<PlannedTimetableInstance> = {},
): PlannedTimetableInstance {
  return {
    id: "1",
    title: "Lernzeit",
    date: "2026-09-08",
    startTime: "10:00",
    endTime: "11:00",
    roomId: "7",
    roomName: "OGS-Raum 1",
    status: "planned",
    isOverdue: false,
    minutesUntilStart: 10,
    ...overrides,
  } as PlannedTimetableInstance;
}

describe("deriveOwnNow (#2180)", () => {
  it("nennt den laufenden Einsatz und den nächsten dahinter", () => {
    const state = deriveOwnNow(
      [
        assignment({ instanceId: "1", startTime: "10:00", endTime: "11:00" }),
        assignment({
          instanceId: "2",
          startTime: "12:00",
          endTime: "13:00",
          title: "Mittagessen",
        }),
      ],
      "10:20",
    );

    expect(state?.kind).toBe("own_running");
    if (state?.kind !== "own_running") throw new Error("unexpected");
    expect(state.block.title).toBe("Lernzeit");
    expect(state.next?.title).toBe("Mittagessen");
  });

  it("nennt sonst den nächsten Einsatz mit der Zeit bis dahin", () => {
    const state = deriveOwnNow(
      [assignment({ startTime: "12:00", endTime: "13:00" })],
      "10:20",
    );

    expect(state).toMatchObject({ kind: "own_next", minutesAhead: 100 });
  });

  // Ausgefallene Einsätze und solche, von denen die Person abgezogen wurde,
  // sind kein „jetzt" — sie stehen mit Etikett in „Mein Tag".
  it("zählt ausgefallene und abgezogene Einsätze nicht", () => {
    const state = deriveOwnNow(
      [
        assignment({ instanceId: "1", cancelled: true }),
        assignment({ instanceId: "2", isAbsent: true }),
      ],
      "10:20",
    );

    expect(state).toBeNull();
  });

  it("sagt am Ende des Tages, dass alles erledigt ist", () => {
    const state = deriveOwnNow(
      [assignment({ startTime: "08:00", endTime: "09:00" })],
      "16:00",
    );

    expect(state).toEqual({ kind: "own_done", count: 1 });
  });
});

describe("deriveSchoolNow (#2180)", () => {
  it("zählt laufende und nicht gestartete Blöcke und nennt den nächsten", () => {
    const state = deriveSchoolNow(
      [
        block({ id: "1", status: "active" }),
        block({ id: "2", status: "active" }),
        block({
          id: "3",
          startTime: "10:00",
          endTime: "11:00",
          isOverdue: true,
        }),
        block({
          id: "4",
          startTime: "12:00",
          endTime: "13:00",
          title: "Mittag",
        }),
        block({ id: "5", status: "cancelled" }),
        block({ id: "6", status: "completed" }),
      ],
      "10:20",
    );

    expect(state.running).toBe(2);
    expect(state.notStarted).toBe(1);
    expect(state.next?.title).toBe("Mittag");
    expect(state.minutesAhead).toBe(100);
    // Ausgefallene Blöcke sind kein Teil des Tages, den man zählt.
    expect(state.total).toBe(5);
  });
});

describe("deriveHomeNow (#2180)", () => {
  // Wer beides ist, sieht den eigenen Tag, solange sie heute Einsätze hat.
  it("zieht den eigenen Tag der Schule vor", () => {
    expect(
      deriveHomeNow({
        now: "10:20",
        own: [assignment()],
        school: [block({ status: "active" })],
      }).kind,
    ).toBe("own_running");
  });

  it("fällt ohne eigene Einsätze auf die Schule zurück", () => {
    expect(
      deriveHomeNow({ now: "10:20", own: [], school: [block()] }).kind,
    ).toBe("school");
  });

  it("bleibt ohne beides bei Uhrzeit und Aktionen", () => {
    expect(
      deriveHomeNow({ now: "10:20", own: undefined, school: undefined }).kind,
    ).toBe("plain");
  });
});

describe("nowActions (#2180)", () => {
  const tenantPath = (path: string) => `/t${path}`;
  const base = {
    isSupervising: false,
    canOpenGroup: false,
    canReadUsers: true,
    tenantPath,
  };

  it("stellt eine laufende Aufsicht an die erste Stelle", () => {
    const actions = nowActions({
      ...base,
      isSupervising: true,
      canOpenGroup: true,
    });

    expect(actions.map((a) => a.label)).toEqual([
      "Aufsicht fortsetzen",
      "Alle Kinder",
    ]);
  });

  // Der Tagesplan ist kein Weg der Zone: „Mein Tag" steht direkt darunter.
  // Alle Kinder vor der eigenen Gruppe: die Gruppe steht schon als Baustein
  // unter der Zone und war als schwarzer Hauptknopf zu präsent.
  it("führt sonst zu allen Kindern, dann in die eigene Gruppe", () => {
    const actions = nowActions({ ...base, canOpenGroup: true });

    expect(actions.map((a) => a.href)).toEqual([
      "/t/students/search",
      "/t/ogs-groups",
    ]);
  });

  it("führt ohne users:read nur in die eigene Gruppe", () => {
    const actions = nowActions({
      ...base,
      canOpenGroup: true,
      canReadUsers: false,
    });

    expect(actions.map((a) => a.href)).toEqual(["/t/ogs-groups"]);
  });

  // Ohne Gruppe bleibt die Kindersuche, wenn die Person sie öffnen darf.
  it("bietet als letzten Weg alle Kinder an", () => {
    const actions = nowActions(base);

    expect(actions).toEqual([
      { href: "/t/students/search", label: "Alle Kinder" },
    ]);
  });

  it("bietet ohne users:read keinen unerlaubten Ersatzweg an", () => {
    expect(nowActions({ ...base, canReadUsers: false })).toEqual([]);
  });

  it("zeigt nie mehr als zwei Wege", () => {
    const actions = nowActions({
      ...base,
      isSupervising: true,
      canOpenGroup: true,
    });

    expect(actions).toHaveLength(2);
  });
});
