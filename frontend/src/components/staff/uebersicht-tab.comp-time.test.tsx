import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { StaffAbsenceRow } from "~/lib/staff-api";

let absences: StaffAbsenceRow[] = [];

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    if (key?.startsWith("staff-absences-account-")) {
      return { data: absences, isLoading: false, error: undefined };
    }
    if (key?.startsWith("staff-history-account-")) {
      return { data: [], isLoading: false, error: undefined };
    }
    if (key?.startsWith("staff-balance-adjustments-")) {
      return { data: [], isLoading: false, error: undefined };
    }
    if (key?.startsWith("staff-schedule-targets-account-")) {
      return { data: new Map(), isLoading: false, error: undefined };
    }
    if (key === "time-tracking-config") {
      return {
        data: { accountStartDate: "2025-01-01" },
        isLoading: false,
        error: undefined,
      };
    }
    return { data: undefined, isLoading: false, error: undefined };
  },
  useTenantMutateMatching: () => () => Promise.resolve([]),
}));

vi.mock("~/lib/staff-api", () => ({
  staffAbsenceService: { getAbsences: vi.fn() },
  staffBalanceAdjustmentService: { list: vi.fn() },
  staffHistoryService: { getHistory: vi.fn() },
  staffMonthSummaryService: {
    getScheduleTargetsRange: vi.fn(),
    getMonthSummary: vi.fn(),
  },
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: vi.fn(), error: vi.fn(), info: vi.fn() }),
}));

vi.mock("~/lib/time-tracking-api", () => ({
  timeTrackingService: { getConfig: vi.fn() },
}));

vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => "2025-03-10",
}));

import { UebersichtTab } from "./uebersicht-tab";

describe("UebersichtTab Freizeitausgleich-Verteilung", () => {
  beforeEach(() => {
    absences = [
      {
        id: 1,
        staff_id: 1,
        absence_type: "comp_time",
        date_start: "2025-03-05",
        date_end: "2025-03-05",
        half_day: true,
        start_half_day: false,
        end_half_day: false,
        note: "",
        status: "reported",
      },
    ];
  });

  it("zählt einen halben Freizeitausgleichstag als 0,5 Tage", () => {
    render(<UebersichtTab staffId="1" />);

    expect(screen.getByText("0.5 Tage")).toBeVisible();
  });
});
