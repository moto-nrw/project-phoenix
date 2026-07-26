import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

// Ein fehlgeschlagener Targets-Fetch darf NICHT auf leere Sollzeiten
// durchfallen: das bepreiste jeden Vertragstag mit 0 Soll und zeichnete eine
// Saldo-Linie, die wie ein riesiger Überschuss aussieht. Stattdessen wird der
// Fehler angezeigt (#1842).
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    if (typeof key === "string" && key.includes("staff-schedule-targets-")) {
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

describe("UebersichtTab Targets-Fehler", () => {
  it("zeigt einen Fehler statt einer 0-Soll-Auswertung", () => {
    render(<UebersichtTab staffId="1" />);

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText(/Sollzeiten konnten nicht geladen/i)).toBeVisible();
    // Keine Chart-Überschrift => es wird kein irreführender Chart gerendert.
    expect(screen.queryByText(/Tagesvergleich Ist \/ Soll/i)).toBeNull();
  });
});
