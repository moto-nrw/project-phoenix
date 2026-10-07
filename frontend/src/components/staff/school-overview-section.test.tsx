import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

import { SchoolOverviewSection } from "./school-overview-section";

const mutate = vi.hoisted(() => vi.fn());
const useSWRAuth = vi.hoisted(() => vi.fn());
const swrResult = vi.hoisted(() => ({
  current: {
    data: undefined as
      | {
          activeStaffCount: number;
          currentlyClockedIn: number;
          expectedClockedIn: number;
          sickToday: number;
          vacationToday: number;
          sollMinutes: number;
          istMinutes: number;
          deltaMinutes: number;
          saldoSchoolTotalMinutes: number;
        }
      | undefined,
    error: undefined as Error | undefined,
    isLoading: false,
    isValidating: false,
    mutate,
  },
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth,
}));

vi.mock("~/lib/staff-overview-api", () => ({
  staffOverviewService: { getDashboardSummary: vi.fn() },
}));

const summary = {
  activeStaffCount: 20,
  currentlyClockedIn: 12,
  expectedClockedIn: 15,
  sickToday: 2,
  vacationToday: 1,
  sollMinutes: 4800,
  istMinutes: 4620,
  deltaMinutes: -180,
  saldoSchoolTotalMinutes: 600,
};

describe("SchoolOverviewSection", () => {
  beforeEach(() => {
    mutate.mockReset();
    useSWRAuth.mockReset();
    useSWRAuth.mockImplementation(() => swrResult.current);
    swrResult.current = {
      data: undefined,
      error: undefined,
      isLoading: false,
      isValidating: false,
      mutate,
    };
  });

  it("deaktiviert previous-key Daten für Zeitraumwechsel", () => {
    render(<SchoolOverviewSection />);

    expect(useSWRAuth).toHaveBeenCalledWith(
      "staff-dashboard-summary-month",
      expect.any(Function),
      {
        keepPreviousData: false,
        refreshInterval: 60_000,
        revalidateOnFocus: false,
      },
    );

    const weekTab = screen.getByRole("button", { name: "Woche" });
    fireEvent.pointerDown(weekTab, { button: 0, pointerType: "mouse" });
    fireEvent.click(weekTab);
    fireEvent.click(weekTab);

    expect(useSWRAuth).toHaveBeenLastCalledWith(
      "staff-dashboard-summary-week",
      expect.any(Function),
      {
        keepPreviousData: false,
        refreshInterval: 60_000,
        revalidateOnFocus: false,
      },
    );
  });

  it("zeigt Ladefehler mit Wiederholen statt leeren KPI-Werten", async () => {
    swrResult.current = {
      data: undefined,
      error: new ApiError("request failed", 503, {
        code: "general.unavailable",
      }),
      isLoading: false,
      isValidating: false,
      mutate,
    };

    render(<SchoolOverviewSection />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Übersicht der Einrichtung"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Aktive Mitarbeitende")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Stundenkonto der Einrichtung"),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(mutate).toHaveBeenCalledTimes(1);
  });

  it("erklärt die Saldo-Veränderung passend zum gewählten Zeitraum", () => {
    swrResult.current = {
      data: summary,
      error: undefined,
      isLoading: false,
      isValidating: false,
      mutate,
    };

    render(<SchoolOverviewSection />);

    expect(screen.getByTitle(/Summe der Monatssalden/)).toBeInTheDocument();

    const weekTab = screen.getByRole("button", { name: "Woche" });
    fireEvent.pointerDown(weekTab, { button: 0, pointerType: "mouse" });
    fireEvent.click(weekTab);
    fireEvent.click(weekTab);

    expect(
      screen.getByTitle(
        /Saldo-Veränderungen aller Mitarbeitenden in dieser Woche/,
      ),
    ).toBeInTheDocument();
  });
});
