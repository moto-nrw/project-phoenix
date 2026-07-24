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
    adjustmentMinutes: 0,
    adjustments: [],
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

  // A month before the account start is summarized standalone by the backend
  // (full month, zero carry) and its closing value never joins the account
  // chain — so neither carry row may claim it does.
  describe("pre-account month", () => {
    it("suppresses the carry rows instead of presenting the month Saldo as one", () => {
      render(
        <Monatskarte
          summary={makeSummary({
            carryInMinutes: 0,
            closingBalanceMinutes: -960,
          })}
          isLoading={false}
          isPreAccountMonth
          accountStartDate="2026-08-01"
        />,
      );

      expect(screen.queryByText("Übertrag Vormonat")).not.toBeInTheDocument();
      expect(screen.queryByText("Übertrag Monatsende")).not.toBeInTheDocument();
      expect(screen.queryByText("Stundenkonto Stand")).not.toBeInTheDocument();
      // The month's own Saldo is still the honest headline figure.
      expect(screen.getByText("Saldo Monat")).toBeInTheDocument();
      expect(
        screen.getByText(
          /liegt vor dem Beginn des Stundenkontos \(01\.08\.2026\)/,
        ),
      ).toBeInTheDocument();
    });

    it("suppresses 'Stundenkonto Stand' for the current month before a future anchor", () => {
      render(
        <Monatskarte
          summary={makeSummary({
            carryInMinutes: 0,
            closingBalanceMinutes: 480,
          })}
          isLoading={false}
          isCurrentMonth
          isPreAccountMonth
          accountStartDate="2026-09-01"
        />,
      );

      expect(screen.queryByText("Stundenkonto Stand")).not.toBeInTheDocument();
      expect(screen.getByText("Saldo Monat (bis heute)")).toBeInTheDocument();
    });
  });

  // A start LATER this month is not "pre-account" by the month comparison in
  // the tab (same YYYY-MM), yet the account has not begun — the backend answers
  // a clean 0 for the empty span before the start day, which must not surface
  // as "Stundenkonto Stand: 0:00" (#1842).
  describe("same-month future account start", () => {
    it("suppresses 'Stundenkonto Stand' and explains when the account begins", () => {
      render(
        <Monatskarte
          summary={makeSummary({ carryInMinutes: 0, closingBalanceMinutes: 0 })}
          isLoading={false}
          isCurrentMonth
          accountStartsInFuture
          accountStartDate="2026-06-20"
        />,
      );

      expect(screen.queryByText("Stundenkonto Stand")).not.toBeInTheDocument();
      // The month's own figures are still shown honestly.
      expect(screen.getByText("Saldo Monat (bis heute)")).toBeInTheDocument();
      expect(
        screen.getByText(/Das Stundenkonto beginnt am 20\.06\.2026/),
      ).toBeInTheDocument();
    });
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
