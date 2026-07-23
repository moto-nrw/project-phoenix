import { render } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";

// Die Charts dieses Tabs bepreisen Vergangenheit. Sie dürfen ihr Soll NICHT
// aus dem AKTUELLEN Dienstplan ableiten: nach einer Vertragsänderung
// (8h -> 4h) rechneten sie Monate an Historie zu den heutigen Stunden nach und
// widersprachen damit der Stundenkonto-Überschrift direkt darüber, die
// datumsgültig vom Server kommt (#1842). Der Test pinnt die Eingaben: nur die
// datumsgültigen Targets, nie der Plan.
const swrKeys: string[] = [];
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    if (key) swrKeys.push(key);
    return { data: undefined, isLoading: false, error: undefined };
  },
  useTenantMutateMatching: () => () => Promise.resolve([]),
}));

const getSchedule = vi.fn();
const getScheduleTargetsRange = vi.fn();
vi.mock("~/lib/staff-api", () => ({
  staffAbsenceService: { getAbsences: vi.fn() },
  staffBalanceAdjustmentService: { list: vi.fn() },
  staffHistoryService: { getHistory: vi.fn() },
  staffScheduleService: {
    getSchedule: (...args: unknown[]) => getSchedule(...args),
  },
  staffMonthSummaryService: {
    getScheduleTargetsRange: (...args: unknown[]) =>
      getScheduleTargetsRange(...args),
    getMonthSummary: vi.fn(),
  },
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: vi.fn(), error: vi.fn(), info: vi.fn() }),
}));

vi.mock("~/lib/time-tracking-api", () => ({
  timeTrackingService: { getConfig: vi.fn() },
}));

// A fixed Berlin day distinct from the real browser date, so a regression to
// `new Date()` would key the target range against a different day (#1842).
vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => "2025-03-10",
}));

import { UebersichtTab } from "./uebersicht-tab";

describe("UebersichtTab Soll-Quelle", () => {
  beforeEach(() => {
    swrKeys.length = 0;
    getSchedule.mockClear();
    getScheduleTargetsRange.mockClear();
  });

  it("lädt datumsgültige Targets für den Kontozeitraum", () => {
    render(<UebersichtTab staffId="1" />);

    expect(
      swrKeys.some((k) => k.startsWith("staff-schedule-targets-account-1-")),
    ).toBe(true);
  });

  it("fragt den aktuellen Dienstplan gar nicht erst ab", () => {
    render(<UebersichtTab staffId="1" />);

    // Kein Plan-Key => keine Möglichkeit, ihn auf historische Tage anzuwenden.
    expect(swrKeys).not.toContain("staff-schedule-1");
  });

  it("leitet 'heute' aus der Berliner Zeit ab, nicht aus der Browser-Zeitzone", () => {
    render(<UebersichtTab staffId="1" />);

    // yearEndKey = toDateKey(berlinToday): der Kontozeitraum-Key endet auf dem
    // Berliner Tag (2025-03-10), nicht auf dem lokalen new Date().
    expect(
      swrKeys.some(
        (k) =>
          k.startsWith("staff-schedule-targets-account-1-") &&
          k.endsWith("-2025-03-10"),
      ),
    ).toBe(true);
  });

  it("lädt auch zukünftige Stundenkonto-Buchungen für die Verwaltung", () => {
    render(<UebersichtTab staffId="1" />);

    expect(
      swrKeys.some(
        (k) =>
          k.startsWith("staff-balance-adjustments-1-") &&
          k.endsWith("-9999-12-31"),
      ),
    ).toBe(true);
  });
});
