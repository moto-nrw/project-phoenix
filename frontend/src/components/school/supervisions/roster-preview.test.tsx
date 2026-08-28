import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SupervisionRosterPreview } from "./roster-preview";
import type { TimetableRosterRow } from "~/lib/timetable-operations-types";

function row(
  studentId: string,
  studentName: string,
  pickupTime: string | null,
): TimetableRosterRow {
  return {
    studentId,
    studentName,
    schoolClass: "2a",
    groupName: "Sonnengruppe",
    planned: true,
    isUnplanned: false,
    currentlyPresent: false,
    visitId: null,
    status: "expected",
    substatus: null,
    note: null,
    checkedInAt: null,
    checkedOutAt: null,
    visitEntryTime: null,
    pickupTime,
    warnings: [],
    careDayStatus: "scheduled",
    parallelPresentIn: null,
  };
}

describe("SupervisionRosterPreview pickup times", () => {
  it("shows the block-date pickup time and the neutral empty placeholder", () => {
    render(
      <SupervisionRosterPreview
        rows={[
          row("1", "Mia Mitzeit", "13:30"),
          row("2", "Nora Ohnezeit", null),
        ]}
        pickupTimesLoaded
        onOpenStudent={vi.fn()}
      />,
    );

    expect(screen.getByText("Gehzeit: 13:30")).toBeInTheDocument();
    expect(screen.getByText("Gehzeit: —")).toBeInTheDocument();
  });

  it("reports a failed lookup without disabling the child list", () => {
    render(
      <SupervisionRosterPreview
        rows={[row("3", "Weiter lesbar", null)]}
        pickupTimesLoaded={false}
        onOpenStudent={vi.fn()}
      />,
    );

    expect(
      screen.getByText(
        "Die Gehzeiten konnten nicht geladen werden. Die Anwesenheitsliste bleibt verfügbar.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Gehzeit: Nicht geladen")).toBeInTheDocument();
    expect(screen.queryByText("Gehzeit: —")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Weiter lesbar/ })).toBeEnabled();
  });
});
