import { describe, expect, it } from "vitest";

import type { ClassDayReport, ClassDayRow } from "~/lib/class-day-api";
import {
  countDayChanges,
  isDayChange,
  isReportedToday,
  reportedTodayLabel,
} from "./day-changes";

function row(overrides: Partial<ClassDayRow> = {}): ClassDayRow {
  return {
    student_id: 1,
    first_name: "Lena",
    last_name: "Meyer",
    registered: true,
    stays_today: true,
    offerings: ["Ganztag"],
    ...overrides,
  };
}

function report(rows: ClassDayRow[], schoolClass = "2a"): ClassDayReport {
  return {
    school_class: schoolClass,
    date: "2026-08-24",
    weekday: "mon",
    school_day: true,
    enrollment_known: true,
    totals: {
      students: rows.length,
      staying: 0,
      leaving: 0,
      absent: 0,
      list_entries: 0,
    },
    rows,
  };
}

describe("isDayChange", () => {
  it("nimmt ein Kind mit gemeldetem Status auf", () => {
    expect(isDayChange(row({ status: "sick" }))).toBe(true);
  });

  it("nimmt ein Kind mit abweichender Abholzeit auf", () => {
    expect(isDayChange(row({ pickup_changed: true }))).toBe(true);
  });

  it("lässt den unveränderten Regelfall draußen", () => {
    expect(isDayChange(row({ pickup: "15:00" }))).toBe(false);
  });

  it("lässt Klassenlisteneinträge draußen", () => {
    // Ein Kind ohne OGS-Datensatz hat keinen Plan, von dem es abweichen
    // könnte — es zählt nie als Abweichung.
    expect(
      isDayChange(row({ list_entry: true, student_id: 0, status: "sick" })),
    ).toBe(false);
  });
});

describe("countDayChanges", () => {
  it("zählt Status und geänderte Abholzeiten zusammen", () => {
    const count = countDayChanges(
      report([
        row({ student_id: 1, status: "sick" }),
        row({ student_id: 2, pickup_changed: true }),
        row({ student_id: 3, pickup: "15:00" }),
        row({ student_id: 0, list_entry: true }),
      ]),
    );

    expect(count).toBe(2);
  });

  it("zählt an einem Tag ohne Übergabe nichts", () => {
    // Wochenende: keine Abweichungen, nicht "unbekannt viele".
    const weekend = { ...report([row({ status: "sick" })]), school_day: false };

    expect(countDayChanges(weekend)).toBe(0);
  });

  it("verträgt eine Klasse, die nicht geladen werden konnte", () => {
    expect(countDayChanges(null)).toBe(0);
  });
});

describe("isReportedToday", () => {
  const now = new Date("2026-08-24T10:00:00Z");

  it("erkennt die Meldung vom selben Berliner Kalendertag", () => {
    expect(isReportedToday("2026-08-24T07:24:00Z", now)).toBe(true);
  });

  it("erkennt eine ältere Meldung", () => {
    expect(isReportedToday("2026-08-20T07:24:00Z", now)).toBe(false);
  });

  it("verträgt fehlende und kaputte Zeitstempel", () => {
    expect(isReportedToday(null, now)).toBe(false);
    expect(isReportedToday(undefined, now)).toBe(false);
    expect(isReportedToday("nicht-datum", now)).toBe(false);
  });

  it("rechnet in Berliner Zeit, nicht in UTC", () => {
    // 23:30 UTC am 23.08. ist in Berlin bereits der 24.08.
    expect(isReportedToday("2026-08-23T23:30:00Z", now)).toBe(true);
    // 22:30 UTC am 24.08. ist in Berlin schon der 25.08.
    expect(isReportedToday("2026-08-24T22:30:00Z", now)).toBe(false);
  });
});

describe("reportedTodayLabel", () => {
  const now = new Date("2026-08-24T10:00:00Z");

  it("nennt die Uhrzeit einer heute eingegangenen Meldung", () => {
    // Das ist die Meldung, die die Lehrkraft noch nicht kennt.
    expect(reportedTodayLabel("2026-08-24T07:24:00Z", now)).toBe(
      "Heute 09:24 gemeldet",
    );
  });

  it("schweigt bei älteren Meldungen", () => {
    // Bei einer vor zwei Wochen geplanten Klassenfahrt beantwortet der
    // Zeitpunkt keine Frage und macht die Zeile nur länger.
    expect(reportedTodayLabel("2026-08-10T09:00:00Z", now)).toBeNull();
    expect(reportedTodayLabel(undefined, now)).toBeNull();
  });
});
