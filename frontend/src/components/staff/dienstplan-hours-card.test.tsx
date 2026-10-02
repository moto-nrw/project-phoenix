import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type {
  StaffScheduleStaff,
  StaffWeeklySummary,
} from "~/lib/shift-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";

import { DienstplanHoursCard } from "./dienstplan-hours-card";

const anna: StaffScheduleStaff = {
  id: "7",
  firstName: "Anna",
  lastName: "Müller",
};
const deniz: StaffScheduleStaff = {
  id: "8",
  firstName: "Deniz",
  lastName: "Kaya",
};
const ohne: StaffScheduleStaff = {
  id: "9",
  firstName: "Ohne",
  lastName: "Dienst",
};

function shiftType(id: string, name: string): ShiftType {
  return { id, name, color: "#83CD2D", description: "", isActive: true };
}

// Legend order, as the Dienstplan lists the Schichtarten.
const SHIFT_TYPES = [
  shiftType("2", "Wochenstunden Ganztag"),
  shiftType("4", "Verfügungsstunden"),
  shiftType("5", "Vertretungsunterricht"),
  shiftType("6", "Pause"),
];

function summary(
  staffId: string,
  overrides: Partial<StaffWeeklySummary>,
): StaffWeeklySummary {
  return {
    staffId,
    weekStart: "2026-09-28",
    plannedMinutes: 0,
    targetMinutes: null,
    deltaMinutes: null,
    plannedByShiftType: [],
    ...overrides,
  };
}

function renderCard(summaries: StaffWeeklySummary[]) {
  return render(
    <DienstplanHoursCard
      staff={[deniz, anna, ohne]}
      summaryByStaff={new Map(summaries.map((s) => [s.staffId, s]))}
      shiftTypes={SHIFT_TYPES}
    />,
  );
}

function tableRow(name: string): HTMLElement {
  const table = screen.getByTestId("data-table-table");
  const row = within(table).getByText(name).closest("tr");
  if (!row) throw new Error(`no table row for ${name}`);
  return row;
}

function cells(row: HTMLElement): string[] {
  return within(row)
    .getAllByRole("cell")
    .map((cell) => cell.textContent ?? "");
}

describe("DienstplanHoursCard", () => {
  it("splits each person's week by Schichtart against the target", () => {
    renderCard([
      summary("7", {
        plannedMinutes: 1290,
        targetMinutes: 1320,
        deltaMinutes: -30,
        plannedByShiftType: [
          { shiftTypeId: "2", plannedMinutes: 1080 },
          { shiftTypeId: "5", plannedMinutes: 120 },
          { shiftTypeId: null, plannedMinutes: 90 },
        ],
      }),
      summary("8", {
        plannedMinutes: 1620,
        targetMinutes: 1500,
        deltaMinutes: 120,
        plannedByShiftType: [
          { shiftTypeId: "2", plannedMinutes: 1500 },
          { shiftTypeId: "4", plannedMinutes: 120 },
        ],
      }),
    ]);

    expect(screen.getByText("Stunden der Woche")).toBeInTheDocument();
    const table = screen.getByTestId("data-table-table");
    // Only the Schichtarten used this week, in legend order; the shifts
    // without one come last. "Pause" is not planned, so it has no column.
    expect(
      within(table)
        .getAllByRole("columnheader")
        // Sortable headers carry a hidden sort hint and an arrow.
        .map((header) =>
          (header.textContent ?? "").replace(/ – Spalte sortieren|[↕↑↓]/g, ""),
        ),
    ).toEqual([
      "Person",
      "Wochenstunden Ganztag",
      "Verfügungsstunden",
      "Vertretungsunterricht",
      "Ohne Schichtart",
      "Gesamt",
      "Soll",
      "Differenz",
    ]);

    expect(cells(tableRow("Müller, Anna"))).toEqual([
      "Müller, Anna",
      "18 h",
      "–",
      "2 h",
      "1,5 h",
      "21,5 h",
      "22 h",
      "−0,5 h",
    ]);
    expect(cells(tableRow("Kaya, Deniz"))).toEqual([
      "Kaya, Deniz",
      "25 h",
      "2 h",
      "–",
      "–",
      "27 h",
      "25 h",
      "+2 h",
    ]);
    // A person without a summary this week gets no row.
    expect(within(table).queryByText("Dienst, Ohne")).not.toBeInTheDocument();
  });

  // Many people only carry a target in a week; sorting by Differenz brings
  // the ones furthest under target to the top.
  it("sorts by Differenz", () => {
    renderCard([
      summary("7", {
        plannedMinutes: 600,
        targetMinutes: 480,
        deltaMinutes: 120,
      }),
      summary("8", {
        plannedMinutes: 0,
        targetMinutes: 480,
        deltaMinutes: -480,
      }),
      summary("9", { plannedMinutes: 60 }),
    ]);
    const table = screen.getByTestId("data-table-table");

    fireEvent.click(within(table).getByRole("button", { name: /Differenz/ }));

    const names = within(table)
      .getAllByRole("row")
      .slice(1)
      .map((row) => within(row).getAllByRole("cell")[0]?.textContent);
    expect(names).toEqual(["Kaya, Deniz", "Müller, Anna", "Dienst, Ohne"]);
  });

  it("leaves Soll and Differenz empty when no target resolves", () => {
    renderCard([
      summary("7", {
        plannedMinutes: 1215,
        plannedByShiftType: [{ shiftTypeId: "2", plannedMinutes: 1215 }],
      }),
    ]);

    expect(cells(tableRow("Müller, Anna"))).toEqual([
      "Müller, Anna",
      "20,25 h",
      "20,25 h",
      "–",
      "–",
    ]);
  });

  // A Schichtart the list no longer knows (deleted, or the list failed to
  // load) still gets its own column instead of vanishing from the sum.
  it("keeps minutes of an unknown Schichtart visible", () => {
    renderCard([
      summary("7", {
        plannedMinutes: 60,
        plannedByShiftType: [{ shiftTypeId: "99", plannedMinutes: 60 }],
      }),
    ]);

    expect(
      within(screen.getByTestId("data-table-table")).getByRole("columnheader", {
        name: "Unbekannte Schichtart",
      }),
    ).toBeInTheDocument();
  });

  it("renders nothing for a week without summaries", () => {
    const { container } = renderCard([]);

    expect(container).toBeEmptyDOMElement();
  });
});
