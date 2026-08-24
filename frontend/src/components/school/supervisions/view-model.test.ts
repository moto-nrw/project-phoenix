import { describe, expect, it } from "vitest";

import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";
import {
  AUTO_VIEW,
  resolveSupervisionView,
  supervisionStartState,
  upcomingAfter,
} from "./view-model";

function instance(
  id: string,
  status: PlannedTimetableInstance["status"],
  startTime = "11:00",
): PlannedTimetableInstance {
  return {
    id,
    title: `Block ${id}`,
    date: "2026-08-24",
    startTime,
    endTime: "12:30",
    roomId: "1",
    roomName: "OGS-Raum 1",
    status,
    isOverdue: false,
    minutesUntilStart: 0,
    expectedStudentsCount: 0,
    presentStudentsCount: 0,
    notScheduledStudentsCount: 0,
    assignedStaffIds: [],
    isAssigned: true,
    isPrimary: true,
    isSubstitute: false,
    isAbsent: false,
    rosterPreview: [],
  };
}

describe("resolveSupervisionView", () => {
  it("zeigt den Überblick, wenn es nichts gibt", () => {
    expect(resolveSupervisionView(AUTO_VIEW, [])).toEqual({ mode: "overview" });
  });

  it("zeigt die laufende Aufsicht und den Weg zu den anderen", () => {
    const list = [instance("1", "planned", "08:00"), instance("2", "active")];
    expect(resolveSupervisionView(AUTO_VIEW, list)).toEqual({
      mode: "detail",
      instance: list[1],
      canGoBack: true,
    });
  });

  it("bietet bei einer einzigen Aufsicht keinen Überblick an", () => {
    const list = [instance("2", "active")];
    expect(resolveSupervisionView(AUTO_VIEW, list)).toEqual({
      mode: "detail",
      instance: list[0],
      canGoBack: false,
    });
  });

  it("zeigt den Überblick, solange keine läuft", () => {
    const list = [instance("1", "planned"), instance("2", "completed")];
    expect(resolveSupervisionView(AUTO_VIEW, list)).toEqual({
      mode: "overview",
    });
  });

  it("öffnet eine angetippte Aufsicht mit Weg zurück", () => {
    const list = [instance("1", "planned"), instance("2", "active")];
    expect(resolveSupervisionView({ kind: "detail", id: "1" }, list)).toEqual({
      mode: "detail",
      instance: list[0],
      canGoBack: true,
    });
  });

  it("faellt auf den Überblick zurück, wenn die geöffnete Aufsicht verschwindet", () => {
    const list = [instance("1", "planned")];
    expect(
      resolveSupervisionView({ kind: "detail", id: "entzogen" }, list),
    ).toEqual({ mode: "overview" });
  });

  it("zeigt den Überblick auch bei laufender Aufsicht, wenn danach gefragt wird", () => {
    const list = [instance("2", "active")];
    expect(resolveSupervisionView({ kind: "overview" }, list)).toEqual({
      mode: "overview",
    });
  });
});

describe("upcomingAfter", () => {
  it("nennt nur spaetere geplante Aufsichten", () => {
    const running = instance("2", "active", "11:00");
    const list = [
      instance("1", "completed", "08:00"),
      running,
      instance("3", "planned", "12:30"),
      instance("4", "cancelled", "14:00"),
    ];
    expect(upcomingAfter(running, list).map((item) => item.id)).toEqual(["3"]);
  });
});

describe("supervisionStartState", () => {
  const now = new Date("2026-08-24T13:20:00+02:00");

  function withWindow(
    base: PlannedTimetableInstance,
    availableAt?: string,
    expiresAt?: string,
  ): PlannedTimetableInstance {
    return {
      ...base,
      canStart: false,
      startAvailableAt: availableAt,
      startExpiresAt: expiresAt,
    };
  }

  it("meldet startbar, solange der Server nicht widerspricht", () => {
    expect(
      supervisionStartState(
        { ...instance("1", "planned"), canStart: true },
        now,
      ),
    ).toBe("startable");
  });

  it("unterscheidet 'noch nicht' von 'nicht mehr'", () => {
    const early = withWindow(
      instance("1", "planned"),
      "2026-08-24T15:00:00+02:00",
      "2026-08-24T16:00:00+02:00",
    );
    expect(supervisionStartState(early, now)).toBe("too_early");

    const over = withWindow(
      instance("1", "planned"),
      "2026-08-24T10:45:00+02:00",
      "2026-08-24T12:30:00+02:00",
    );
    expect(supervisionStartState(over, now)).toBe("expired");
  });

  it("liest ein fehlendes Fenster als vorbei, nicht als 'gleich'", () => {
    expect(
      supervisionStartState(withWindow(instance("1", "planned")), now),
    ).toBe("expired");
  });

  it("meldet abgesagt und beendet vor allem anderen", () => {
    expect(supervisionStartState(instance("1", "cancelled"), now)).toBe(
      "cancelled",
    );
    expect(supervisionStartState(instance("1", "completed"), now)).toBe(
      "completed",
    );
  });
});
