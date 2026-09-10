import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { DashboardAnalytics } from "~/lib/dashboard-helpers";
import type { DashboardSummary } from "~/lib/staff-overview-api";

const swr = vi.hoisted(() => ({
  data: undefined as DashboardSummary | undefined,
  error: undefined as Error | undefined,
  isLoading: false,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => swr,
}));
vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => `/test-tenant${path}`,
}));
vi.mock("~/lib/staff-overview-api", () => ({
  staffOverviewService: { getDashboardSummary: vi.fn() },
}));

import { StaffTodayBlock, staffTiles } from "./staff-today-block";

function summary(overrides: Partial<DashboardSummary> = {}): DashboardSummary {
  return {
    activeStaffCount: 12,
    sickToday: 1,
    vacationToday: 2,
    currentlyClockedIn: 6,
    expectedClockedIn: 8,
    sollMinutes: 0,
    istMinutes: 0,
    deltaMinutes: 0,
    saldoSchoolTotalMinutes: 0,
    pendingRequestsCount: 3,
    ...overrides,
  };
}

const analytics = { supervisorsToday: 7 } as DashboardAnalytics;

describe("StaffTodayBlock (#2180)", () => {
  beforeEach(() => {
    swr.data = summary();
    swr.error = undefined;
    swr.isLoading = false;
  });

  it("zeigt Aufsicht, Ausfälle und Stempeluhr", () => {
    render(<StaffTodayBlock analytics={analytics} />);

    expect(screen.getByText("In Aufsicht")).toBeInTheDocument();
    expect(screen.getByText("7")).toBeInTheDocument();
    expect(screen.getByText("Krank")).toBeInTheDocument();
    expect(screen.getByText("Im Urlaub")).toBeInTheDocument();
    expect(screen.getByText("6 von 8")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Personal heute: zum Team" }),
    ).toHaveAttribute("href", "/test-tenant/staff");
  });

  // Die Stempeluhr steht nur da, wo sie benutzt wird; ohne Betriebszahlen
  // fehlt die Aufsicht, und das Team gesamt füllt den Platz.
  it("lässt weg, was die Schule nicht nutzt", () => {
    const tiles = staffTiles(
      summary({ currentlyClockedIn: 0, expectedClockedIn: 0 }),
      undefined,
    );

    expect(tiles.map((tile) => tile.label)).toEqual([
      "Krank",
      "Im Urlaub",
      "Team gesamt",
    ]);
  });

  it("zeigt nie mehr als vier Zahlen", () => {
    expect(staffTiles(summary(), analytics)).toHaveLength(4);
  });

  it("unterscheidet einen Ladefehler von Nullen", () => {
    swr.data = undefined;
    swr.error = new Error("boom");

    render(<StaffTodayBlock analytics={analytics} />);

    expect(screen.getByText(/konnte nicht geladen werden/)).toBeInTheDocument();
    expect(screen.queryByText("Krank")).not.toBeInTheDocument();
  });
});
