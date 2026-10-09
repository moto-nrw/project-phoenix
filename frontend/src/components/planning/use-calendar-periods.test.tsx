import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  mockListPeriods,
  mockListPhases,
  mockSetPhaseCalendarPeriod,
  mockToastError,
  mockToastSuccess,
} = vi.hoisted(() => ({
  mockListPeriods: vi.fn(),
  mockListPhases: vi.fn(),
  mockSetPhaseCalendarPeriod: vi.fn(),
  mockToastError: vi.fn(),
  mockToastSuccess: vi.fn(),
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: mockToastSuccess, error: mockToastError }),
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
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("confirms a phase link in a full sentence", async () => {
    mockSetPhaseCalendarPeriod.mockResolvedValueOnce(phase);
    const { result } = renderHook(() => useCalendarPeriods());

    await waitFor(() => expect(result.current.loading).toBe(false));
    act(() => result.current.beginEdit(period));

    await act(async () => {
      await result.current.handlePhaseLinkToggle(phase, true);
    });

    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Die Anmeldephase „Demo Anmeldung“ ist mit „Schuljahr 2026/2027“ verknüpft.",
    );
  });

  it("keeps the period list when only the phases fail to load", async () => {
    mockListPhases.mockRejectedValueOnce(new Error("phases down"));
    const { result } = renderHook(() => useCalendarPeriods());

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.periods).toEqual([period]);
    expect(result.current.phases).toEqual([]);
    expect(result.current.loadFailed).toBe(false);
    expect(result.current.error).toBeNull();
  });
});
