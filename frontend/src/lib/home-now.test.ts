import { describe, expect, it } from "vitest";

import type { OwnAssignment } from "~/lib/shift-helpers";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

import {
  deriveHomeNow,
  deriveOwnNow,
  deriveSchoolNow,
  nowActions,
  startableOwnBlock,
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

describe("startableOwnBlock (#2180)", () => {
  const at = new Date("2026-09-08T10:05:00+02:00");

  it("nimmt den eigenen, geplanten Block, den der Server freigibt", () => {
    const mine = block({ id: "4", isAssigned: true, canStart: true });

    expect(startableOwnBlock([mine], at)).toBe(mine);
  });

  it("übergeht fremde, laufende und nicht freigegebene Blöcke", () => {
    expect(
      startableOwnBlock(
        [
          block({ id: "1", isAssigned: false, canStart: true }),
          block({ id: "2", isAssigned: true, status: "active" }),
          block({ id: "3", isAssigned: true, canStart: false }),
          block({
            id: "4",
            isAssigned: true,
            canStart: true,
            startExpiresAt: "2026-09-08T10:00:00+02:00",
          }),
        ],
        at,
      ),
    ).toBeNull();
  });

  it("nimmt bei zwei freigegebenen Blöcken den früheren", () => {
    const later = block({
      id: "5",
      isAssigned: true,
      canStart: true,
      startTime: "10:30",
    });
    const earlier = block({
      id: "6",
      isAssigned: true,
      canStart: true,
      startTime: "10:00",
    });

    expect(startableOwnBlock([later, earlier], at)).toBe(earlier);
  });
});

describe("nowActions (#2180)", () => {
  const tenantPath = (path: string) => `/t${path}`;
  const base = {
    isSupervising: false,
    startable: null,
    canReadUsers: true,
    tenantPath,
  };
  const startable = block({ id: "9", isAssigned: true, canStart: true });

  it("führt bei laufender eigener Aufsicht zu ihr", () => {
    const actions = nowActions({ ...base, isSupervising: true });

    expect(actions.map((a) => a.label)).toEqual([
      "Zur Aufsicht",
      "Alle Kinder",
    ]);
    expect(actions[0]).toMatchObject({ href: "/t/active-supervisions" });
  });

  it("bietet den eigenen Block zum Starten an, solange keine Aufsicht läuft", () => {
    const actions = nowActions({ ...base, startable });

    expect(actions[0]).toEqual({
      kind: "start",
      block: startable,
      label: "Aufsicht starten",
    });
    expect(actions[1]).toMatchObject({
      kind: "link",
      href: "/t/students/search",
    });
  });

  // Ein zweiter Block im Fenster steht mit Starten-Knopf in „Mein Tag".
  it("startet nichts von hier, wenn schon eine Aufsicht läuft", () => {
    const actions = nowActions({ ...base, isSupervising: true, startable });

    expect(actions.map((a) => a.kind)).toEqual(["link", "link"]);
    expect(actions[0]?.label).toBe("Zur Aufsicht");
  });

  // Weder die eigene Gruppe (steht als Baustein darunter) noch der Tagesplan
  // („Mein Tag" steht darunter) sind Wege der Zone.
  it("führt sonst nur zu allen Kindern", () => {
    expect(nowActions(base)).toEqual([
      { kind: "link", href: "/t/students/search", label: "Alle Kinder" },
    ]);
  });

  it("bietet ohne users:read keinen unerlaubten Ersatzweg an", () => {
    expect(nowActions({ ...base, canReadUsers: false })).toEqual([]);
  });

  it("zeigt nie mehr als zwei Wege", () => {
    expect(
      nowActions({ ...base, isSupervising: true, startable }),
    ).toHaveLength(2);
  });
});
