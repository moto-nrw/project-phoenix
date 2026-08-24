import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { ClassDayRow } from "~/lib/class-day-api";
import { StudentRow } from "./student-row";

const NOW = new Date("2026-08-24T10:00:00Z");

function row(overrides: Partial<ClassDayRow> = {}): ClassDayRow {
  return {
    student_id: 1,
    first_name: "Emilia",
    last_name: "Braun",
    registered: true,
    stays_today: true,
    offerings: ["Ganztag"],
    ...overrides,
  };
}

function renderRow(overrides: Partial<ClassDayRow> = {}) {
  return render(
    <ul>
      <StudentRow row={row(overrides)} enrollmentKnown now={NOW} />
    </ul>,
  );
}

describe("StudentRow", () => {
  it("nennt bei abweichender Abholzeit beide Zeiten", () => {
    // Ohne die Regelzeit daneben liest sich "bis 12:15" wie der Normalfall —
    // genau die Fehllesung, um die es in #2294 geht.
    renderRow({
      pickup: "12:15",
      pickup_regular: "15:30",
      pickup_changed: true,
    });

    expect(screen.getByText("bis 12:15")).toBeInTheDocument();
    expect(screen.getByText("sonst 15:30")).toBeInTheDocument();
    expect(screen.getByText("Andere Abholzeit")).toBeInTheDocument();
  });

  it("nennt eine abweichende Zeit auch, wenn das Kind nicht als bleibend gilt", () => {
    // Sonst trüge die Zeile das Kennzeichen ohne die Zeit, um die es geht.
    renderRow({
      stays_today: false,
      pickup: "12:15",
      pickup_regular: "15:30",
      pickup_changed: true,
    });

    expect(screen.getByText("bis 12:15")).toBeInTheDocument();
    expect(screen.getByText("sonst 15:30")).toBeInTheDocument();
  });

  it("lässt die unveränderte Zeile ohne Kennzeichen und ohne Regelzeit", () => {
    renderRow({ pickup: "15:30" });

    expect(screen.getByText("bis 15:30")).toBeInTheDocument();
    expect(screen.queryByText(/^sonst /)).not.toBeInTheDocument();
    expect(screen.queryByText("Andere Abholzeit")).not.toBeInTheDocument();
  });

  it("zeigt die Meldezeit nur bei Meldungen von heute", () => {
    const { unmount } = renderRow({
      status: "sick",
      reported_at: "2026-08-24T07:24:00Z",
    });
    expect(screen.getByText("Heute 09:24 gemeldet")).toBeInTheDocument();
    unmount();

    // Eine vor zwei Wochen geplante Klassenfahrt braucht keine Uhrzeit.
    renderRow({ status: "class_trip", reported_at: "2026-08-10T09:00:00Z" });
    expect(screen.queryByText(/gemeldet/)).not.toBeInTheDocument();
  });

  it("gibt dem gemeldeten Status den Vorrang vor der geänderten Abholzeit", () => {
    // Beides zugleich: das Kind ist krank. "Andere Abholzeit" wäre die
    // schwächere und irreführende Aussage.
    renderRow({ status: "sick", pickup_changed: true, pickup: "12:15" });

    expect(screen.getByText("Krank")).toBeInTheDocument();
    expect(screen.queryByText("Andere Abholzeit")).not.toBeInTheDocument();
    expect(screen.queryByText("bis 12:15")).not.toBeInTheDocument();
    expect(screen.queryByText(/^sonst /)).not.toBeInTheDocument();
  });

  it("kennzeichnet ein Kind ohne OGS-Datensatz und erfindet keine Abweichung", () => {
    renderRow({
      student_id: 0,
      list_entry: true,
      list_entry_id: "42",
      stays_today: false,
      pickup_changed: true,
    });

    expect(screen.getByText("Keine Betreuung")).toBeInTheDocument();
    expect(screen.queryByText("Andere Abholzeit")).not.toBeInTheDocument();
  });

  it("bleibt reine Anzeige ohne Bedienelemente", () => {
    // Missverständnis-Check: Änderungen macht das OGS-Team, nicht die
    // Lehrkraft an dieser Zeile.
    renderRow({ status: "sick" });

    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
