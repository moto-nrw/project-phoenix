import { describe, expect, it } from "vitest";

import type { ClassDayReport, ClassDayRow } from "~/lib/class-day-api";
import {
  collectDayChanges,
  describeDayChange,
  isDayChange,
  isReportedToday,
} from "./day-changes";
import { statusLabel } from "./status-labels";

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
    // könnte — es gehört nie in den Abweichungsblock.
    expect(
      isDayChange(row({ list_entry: true, student_id: 0, status: "sick" })),
    ).toBe(false);
  });
});

describe("collectDayChanges", () => {
  it("sammelt über alle Klassen und sortiert die jüngste Meldung nach oben", () => {
    const changes = collectDayChanges(["2a", "3b"], {
      "2a": report([
        row({
          student_id: 1,
          last_name: "Adam",
          status: "sick",
          reported_at: "2026-08-20T08:00:00Z",
        }),
        row({
          student_id: 2,
          last_name: "Bosch",
          pickup_changed: true,
          pickup: "12:15",
          pickup_regular: "15:00",
          reported_at: "2026-08-24T09:24:00Z",
        }),
      ]),
      "3b": report(
        [row({ student_id: 3, last_name: "Cerny", status: "excused" })],
        "3b",
      ),
    });

    expect(changes.map((change) => change.row.last_name)).toEqual([
      "Bosch",
      "Adam",
      "Cerny",
    ]);
    expect(changes[0]?.kind).toBe("pickup");
    expect(changes[0]?.schoolClass).toBe("2a");
    // Ohne Zeitstempel hängt die Zeile hinten, nicht irgendwo dazwischen.
    expect(changes[2]?.reportedAt).toBeNull();
  });

  it("überspringt Klassen ohne Schultag", () => {
    const weekend = { ...report([row({ status: "sick" })]), school_day: false };

    expect(collectDayChanges(["2a"], { "2a": weekend })).toEqual([]);
  });

  it("überspringt Klassen, die nicht geladen werden konnten", () => {
    expect(collectDayChanges(["2a", "3b"], {})).toEqual([]);
  });

  it("gibt dem Status den Vorrang vor der geänderten Abholzeit", () => {
    // Beides zugleich: das Kind ist krank. "Andere Abholzeit" wäre die
    // schwächere und irreführende Aussage.
    const changes = collectDayChanges(["2a"], {
      "2a": report([row({ status: "sick", pickup_changed: true })]),
    });

    expect(changes[0]?.kind).toBe("status");
  });
});

describe("describeDayChange", () => {
  const [statusChange] = collectDayChanges(["2a"], {
    "2a": report([row({ status: "sick" })]),
  });
  const [pickupChange] = collectDayChanges(["2a"], {
    "2a": report([
      row({ pickup_changed: true, pickup: "12:15", pickup_regular: "15:00" }),
    ]),
  });
  const [pickupWithoutRegular] = collectDayChanges(["2a"], {
    "2a": report([row({ pickup_changed: true, pickup: "14:00" })]),
  });

  it("benennt den gemeldeten Status", () => {
    expect(describeDayChange(statusChange!, statusLabel)).toBe("Krank");
  });

  it("nennt bei der Abholzeit immer beide Zeiten", () => {
    // Ohne die Regelzeit daneben liest sich "geht um 12:15" wie der
    // Normalfall — genau die Fehllesung, die der Block verhindern soll.
    expect(describeDayChange(pickupChange!, statusLabel)).toBe(
      "Geht um 12:15 Uhr statt um 15:00 Uhr",
    );
  });

  it("lässt die Regelzeit weg, wenn der Plan an dem Tag keine hat", () => {
    expect(describeDayChange(pickupWithoutRegular!, statusLabel)).toBe(
      "Geht um 14:00 Uhr",
    );
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
    expect(isReportedToday("nicht-datum", now)).toBe(false);
  });

  it("rechnet in Berliner Zeit, nicht in UTC", () => {
    // 23:30 UTC am 23.08. ist in Berlin bereits der 24.08. — eine abends
    // gemeldete Änderung darf am Folgetag nicht als "heute" erscheinen.
    expect(isReportedToday("2026-08-23T23:30:00Z", now)).toBe(true);
    expect(
      isReportedToday("2026-08-24T22:30:00Z", new Date("2026-08-24T10:00:00Z")),
    ).toBe(false);
  });
});
