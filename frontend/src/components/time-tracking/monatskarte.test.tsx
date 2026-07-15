import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { MonthSummary } from "~/lib/time-tracking-helpers";
import { Monatskarte, monthLabel } from "./monatskarte";

function makeSummary(overrides: Partial<MonthSummary> = {}): MonthSummary {
  return {
    staffId: "42",
    year: 2026,
    month: 6,
    carryInMinutes: -90,
    targetMinutes: 2400,
    targetMinutesToDate: 2400,
    actualMinutes: 720,
    creditedSickMinutes: 480,
    creditedVacationMinutes: 240,
    creditedOtherMinutes: 0,
    sickDays: 1,
    vacationDays: 0.5,
    plannedShiftMinutes: 180,
    balanceMinutes: -960,
    closingBalanceMinutes: -1050,
    ...overrides,
  };
}

describe("Monatskarte", () => {
  it("renders the month aggregate with credits and coverage", () => {
    render(<Monatskarte summary={makeSummary()} isLoading={false} />);

    expect(screen.getByText("Monatskarte Juni 2026")).toBeInTheDocument();
    expect(screen.getByText("Übertrag Vormonat")).toBeInTheDocument();
    expect(screen.getByText("Gutschrift Krankheit")).toBeInTheDocument();
    expect(screen.getByText("1 Tag")).toBeInTheDocument();
    expect(screen.getByText("Gutschrift Urlaub")).toBeInTheDocument();
    expect(screen.getByText("0,5 Tage")).toBeInTheDocument();
    // Coverage hint: 180 planned vs 2400 target → under-planned.
    expect(screen.getByText(/unter Soll verplant/)).toBeInTheDocument();
    // Past month → final row reads as Übertrag.
    expect(screen.getByText("Übertrag Monatsende")).toBeInTheDocument();
  });

  it("shows 'kein Dienstplan gepflegt' when no shifts exist", () => {
    render(
      <Monatskarte
        summary={makeSummary({ plannedShiftMinutes: null })}
        isLoading={false}
      />,
    );
    expect(screen.getByText("kein Dienstplan gepflegt")).toBeInTheDocument();
  });

  it("pro-rates labels for the current month", () => {
    render(
      <Monatskarte summary={makeSummary()} isLoading={false} isCurrentMonth />,
    );
    expect(screen.getByText(/davon bis heute/)).toBeInTheDocument();
    expect(screen.getByText("Saldo Monat (bis heute)")).toBeInTheDocument();
    expect(screen.getByText("Stundenkonto Stand")).toBeInTheDocument();
  });

  it("renders nothing without a summary and an error state on failure", () => {
    const { container, rerender } = render(
      <Monatskarte summary={null} isLoading={false} />,
    );
    expect(container).toBeEmptyDOMElement();

    rerender(
      <Monatskarte
        summary={null}
        isLoading={false}
        error="Fehler beim Laden"
      />,
    );
    expect(screen.getByText("Fehler beim Laden")).toBeInTheDocument();
  });
});

describe("monthLabel", () => {
  it("formats German month names", () => {
    expect(monthLabel(2026, 1)).toBe("Januar 2026");
    expect(monthLabel(2026, 12)).toBe("Dezember 2026");
  });
});
