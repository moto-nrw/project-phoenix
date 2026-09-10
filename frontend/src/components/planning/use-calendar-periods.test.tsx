import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  mockListPeriods,
  mockListPhases,
  mockSetPhaseCalendarPeriod,
  mockToastError,
} = vi.hoisted(() => ({
  mockListPeriods: vi.fn(),
  mockListPhases: vi.fn(),
  mockSetPhaseCalendarPeriod: vi.fn(),
  mockToastError: vi.fn(),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: vi.fn(), error: mockToastError }),
}));

vi.mock("~/lib/calendar-period-api", () => ({
  calendarPeriodService: { list: mockListPeriods },
}));

vi.mock("~/lib/enrollment-phase-api", () => ({
  listPhases: mockListPhases,
  setPhaseCalendarPeriod: mockSetPhaseCalendarPeriod,
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), warn: vi.fn() }),
}));

import { useCalendarPeriods } from "./use-calendar-periods";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import type { Phase } from "~/lib/enrollment-phase-api";

const period: CalendarPeriod = {
  id: "5",
  tenantId: "1",
  name: "Schuljahr 2026/2027",
  periodType: "school_year",
  startDate: "2026-08-01",
  endDate: "2027-07-31",
  weekCycleLength: 1,
  weekCycleAnchor: null,
  isActive: true,
  createdAt: "2026-05-01T00:00:00Z",
  updatedAt: "2026-05-01T00:00:00Z",
};

const phase = {
  id: "7",
  name: "Demo Anmeldung",
  calendar_period_id: null,
  is_active: true,
} as Phase;

describe("useCalendarPeriods", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListPeriods.mockResolvedValue([period]);
    mockListPhases.mockResolvedValue([phase]);
  });

  it("propagates a failed phase-link write to the period modal", async () => {
    mockSetPhaseCalendarPeriod.mockRejectedValueOnce(
      new Error("Verknüpfung fehlgeschlagen"),
    );
    const { result } = renderHook(() => useCalendarPeriods());

    await waitFor(() => expect(result.current.loading).toBe(false));
    act(() => result.current.beginEdit(period));

    await act(async () => {
      await expect(
        result.current.handlePhaseLinkToggle(phase, true),
      ).rejects.toThrow("Verknüpfung fehlgeschlagen");
    });

    expect(mockToastError).not.toHaveBeenCalled();
  });
});
