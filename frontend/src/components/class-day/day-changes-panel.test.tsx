import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { ClassDayReport, ClassDayRow } from "~/lib/class-day-api";
import { DayChangesPanel } from "./day-changes-panel";

const NOW = new Date("2026-08-24T10:00:00Z");

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

describe("DayChangesPanel", () => {
  it("nennt bei einer geänderten Abholzeit beide Zeiten", () => {
    render(
      <DayChangesPanel
        classes={["2a"]}
        reports={{
          "2a": report([
            row({
              pickup_changed: true,
              pickup: "12:15",
              pickup_regular: "15:00",
              reported_at: "2026-08-24T07:24:00Z",
            }),
          ]),
        }}
        dateISO="2026-08-24"
        now={NOW}
      />,
    );

    expect(
      screen.getByText("Geht um 12:15 Uhr statt um 15:00 Uhr"),
    ).toBeInTheDocument();
    expect(screen.getByText("Andere Abholzeit")).toBeInTheDocument();
  });

  it("markiert eine heute eingegangene Meldung mit ihrer Uhrzeit", () => {
    render(
      <DayChangesPanel
        classes={["2a"]}
        reports={{
          "2a": report([
            row({ status: "sick", reported_at: "2026-08-24T07:24:00Z" }),
          ]),
        }}
        dateISO="2026-08-24"
        now={NOW}
      />,
    );

    // Kurzfristig heißt: heute hereingekommen. Genau das ist die Meldung,
    // die die Lehrkraft noch nicht kennt.
    expect(screen.getByText("Heute 09:24 gemeldet")).toBeInTheDocument();
  });

  it("nennt bei älteren Meldungen nur das Datum", () => {
    render(
      <DayChangesPanel
        classes={["2a"]}
        reports={{
          "2a": report([
            row({ status: "class_trip", reported_at: "2026-08-10T09:00:00Z" }),
          ]),
        }}
        dateISO="2026-08-24"
        now={NOW}
      />,
    );

    expect(screen.getByText("10.08.2026 gemeldet")).toBeInTheDocument();
  });

  it("sagt ausdrücklich, wenn nichts abweicht", () => {
    // Ein leerer Block wäre nicht von "noch nicht geladen" zu unterscheiden.
    render(
      <DayChangesPanel
        classes={["2a"]}
        reports={{ "2a": report([row({ pickup: "15:00" })]) }}
        dateISO="2026-08-24"
        now={NOW}
      />,
    );

    expect(
      screen.getByText(
        "An diesem Tag weicht in Ihren Klassen nichts vom üblichen Plan ab.",
      ),
    ).toBeInTheDocument();
  });

  it("bleibt reine Anzeige ohne Bedienelemente", () => {
    // Missverständnis-Check: nichts hier ist anklickbar, Änderungen macht
    // das OGS-Team.
    render(
      <DayChangesPanel
        classes={["2a"]}
        reports={{ "2a": report([row({ status: "sick" })]) }}
        dateISO="2026-08-24"
        now={NOW}
      />,
    );

    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("stellt der Zeile die Klasse voran, sobald mehrere Klassen zugewiesen sind", () => {
    render(
      <DayChangesPanel
        classes={["2a", "3b"]}
        reports={{
          "2a": report([row({ last_name: "Meyer", status: "sick" })]),
          "3b": report(
            [row({ student_id: 2, last_name: "Cerny", status: "excused" })],
            "3b",
          ),
        }}
        dateISO="2026-08-24"
        now={NOW}
      />,
    );

    expect(screen.getByText(/2a · Meyer, Lena/)).toBeInTheDocument();
    expect(screen.getByText(/3b · Cerny, Lena/)).toBeInTheDocument();
  });
});
