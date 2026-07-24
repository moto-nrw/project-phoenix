import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

// Ein fehlgeschlagener Buchungs-Fetch darf NICHT als leeres Buchungsprotokoll
// durchgehen: Auszahlungen und Resets fehlten dann sowohl im Protokoll
// ("Noch keine Buchungen") als auch in der Saldo-Kurve. Stattdessen wird der
// Fehler angezeigt (#1420).
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    if (typeof key === "string" && key.includes("staff-balance-adjustments-")) {
      return {
        data: undefined,
        isLoading: false,
        error: new Error("boom"),
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
  staffScheduleService: { getSchedule: vi.fn() },
  staffMonthSummaryService: {
    getScheduleTargetsRange: vi.fn(),
    getMonthSummary: vi.fn(),
  },
}));

vi.mock("~/lib/time-tracking-api", () => ({
  timeTrackingService: { getConfig: vi.fn() },
}));

vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => "2025-03-10",
}));

import { UebersichtTab } from "./uebersicht-tab";

describe("UebersichtTab Buchungs-Fehler", () => {
  it("zeigt einen Fehler statt eines leeren Buchungsprotokolls", () => {
    render(<UebersichtTab staffId="1" />);

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(
      screen.getByText(/Stundenkonto-Buchungen konnten nicht geladen/i),
    ).toBeVisible();
    // Kein Panel => kein irreführendes leeres Protokoll.
    expect(screen.queryByText(/Noch keine Buchungen/i)).toBeNull();
  });
});
