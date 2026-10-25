import { describe, expect, it } from "vitest";

import type { WorkDayLaborHint } from "~/lib/time-tracking-helpers";

import {
  dayLaborHints,
  liveDayLaborHints,
  weekLaborHints,
} from "./labor-time-hints";

function day(overrides: Partial<WorkDayLaborHint> = {}): WorkDayLaborHint {
  return {
    date: "2026-09-09",
    netMinutes: 300,
    breakMinutes: 0,
    countedBreakMinutes: 0,
    requiredBreakMinutes: 0,
    isBreakTooShort: false,
    isOverDailyMax: false,
    isRunning: true,
    overDailyMaxAt: null,
    breakTooShortAt: null,
    breakTooShortRequiredMinutes: 0,
    ...overrides,
  };
}

const now = new Date("2026-09-09T12:00:00+02:00");
const earlier = "2026-09-09T11:59:00+02:00";
const later = "2026-09-09T12:01:00+02:00";

describe("labor-time-hints (#4804)", () => {
  it("nennt nötige und gemachte Pause", () => {
    const hints = dayLaborHints(
      day({
        isBreakTooShort: true,
        requiredBreakMinutes: 30,
        countedBreakMinutes: 15,
      }),
    );
    expect(hints.map((hint) => hint.label)).toEqual([
      "Pause zu kurz: 30 Min. nötig, 15 Min. gemacht",
    ]);
  });

  it("gibt ohne Markierung keinen Hinweis", () => {
    expect(dayLaborHints(day())).toEqual([]);
    expect(dayLaborHints(undefined)).toEqual([]);
  });

  it("wechselt live erst ab dem Zeitpunkt vom Server", () => {
    expect(
      liveDayLaborHints(day({ overDailyMaxAt: later }), now).map((h) => h.key),
    ).toEqual([]);
    expect(
      liveDayLaborHints(day({ overDailyMaxAt: earlier }), now).map(
        (h) => h.label,
      ),
    ).toEqual(["Mehr als 10 Std. gearbeitet"]);
  });

  it("nennt live die Pause der nächsten Stufe", () => {
    const hints = liveDayLaborHints(
      day({
        countedBreakMinutes: 30,
        requiredBreakMinutes: 30,
        breakTooShortAt: earlier,
        breakTooShortRequiredMinutes: 45,
      }),
      now,
    );
    expect(hints.map((hint) => hint.label)).toEqual([
      "Pause zu kurz: 45 Min. nötig, 30 Min. gemacht",
    ]);
  });

  it("markiert eine Woche nur über 48 Stunden", () => {
    const base = {
      weekNumber: 2,
      year: 2026,
      totalNetMinutes: 2940,
      targetMinutes: null,
      deltaMinutes: null,
      sessionCount: 5,
    };
    expect(weekLaborHints({ ...base, isOverWeeklyMax: true })).toHaveLength(1);
    expect(weekLaborHints({ ...base, isOverWeeklyMax: false })).toEqual([]);
  });
});
