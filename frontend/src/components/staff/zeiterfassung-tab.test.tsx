import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { PeriodMetrics } from "~/lib/hooks/use-period-metrics";
import { catalogText } from "~/test/error-catalog-text";

const mocks = vi.hoisted(() => ({
  metrics: null as PeriodMetrics | null,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => ({
    data: undefined,
    error: undefined,
    isLoading: false,
    mutate: vi.fn(),
  }),
  useTenantMutateMatching: () => vi.fn(),
}));

vi.mock("~/lib/hooks/use-period-metrics", () => ({
  usePeriodMetrics: () => mocks.metrics,
}));

vi.mock("./staff-session-table", () => ({
  StaffSessionTable: () => <div data-testid="staff-session-table" />,
  isStaleAfterSessionSave: () => false,
}));

vi.mock("./staff-export-button", () => ({
  StaffExportButton: () => null,
}));

import { ZeiterfassungTab } from "./zeiterfassung-tab";

function metrics(overrides: Partial<PeriodMetrics> = {}): PeriodMetrics {
  return {
    week: { soll: 2340, ist: 1200, delta: -60 },
    month: { soll: 9360, ist: 4800, delta: 120 },
    accountStart: new Date(2026, 4, 13),
    accountBalanceMinutes: 300,
    retry: vi.fn(() => Promise.resolve()),
    ...overrides,
  };
}

describe("ZeiterfassungTab Kennzahlen", () => {
  beforeEach(() => {
    mocks.metrics = metrics();
  });

  it("shows the figures once they are loaded", () => {
    render(<ZeiterfassungTab staffId="42" />);

    // "Diese Woche" also labels the range navigation; these two are cards only.
    expect(screen.getByText("Überstunden Monat")).toBeInTheDocument();
    expect(screen.getByText("Stundenkonto")).toBeInTheDocument();
  });

  // #3885: a failed source must not leave the cards standing with „–“ or a 0
  // that reads like an empty month; only the notice with retry remains.
  it("replaces the cards with the load error and a retry", async () => {
    const retry = vi.fn(() => Promise.resolve());
    mocks.metrics = metrics({
      week: null,
      month: null,
      accountBalanceMinutes: null,
      failed: true,
      error: new ApiError("metrics unavailable", 503, {
        code: "general.unavailable",
        instance: "req-metrics",
      }),
      retry,
    });

    render(<ZeiterfassungTab staffId="42" />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Übersicht der Arbeitszeit"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Dieser Monat")).not.toBeInTheDocument();
    expect(screen.queryByText("Überstunden Monat")).not.toBeInTheDocument();
    expect(screen.queryByText("Stundenkonto")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(retry).toHaveBeenCalledTimes(1);
  });
});
