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
}));

const getSchedule = vi.fn();
const getScheduleTargetsRange = vi.fn();
vi.mock("~/lib/staff-api", () => ({
  staffAbsenceService: { getAbsences: vi.fn() },
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

vi.mock("~/lib/time-tracking-api", () => ({
  timeTrackingService: { getConfig: vi.fn() },
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
});
