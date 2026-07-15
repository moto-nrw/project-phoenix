import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";

const mocks = vi.hoisted(() => ({
  useSession: vi.fn(),
  hasPermission: vi.fn(),
  useSWRAuth: vi.fn(),
  useTenantMutateMatching: vi.fn(),
  getOverview: vi.fn(),
  getShifts: vi.fn(),
  getAllStaff: vi.fn(),
  getShiftTypes: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: mocks.useSession,
}));

vi.mock("~/lib/auth-utils", () => ({
  hasPermission: mocks.hasPermission,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: mocks.useSWRAuth,
  useTenantMutateMatching: mocks.useTenantMutateMatching,
}));

vi.mock("~/lib/shift-api", () => ({
  staffShiftService: {
    getOverview: mocks.getOverview,
    getShifts: mocks.getShifts,
  },
}));

vi.mock("~/lib/staff-api", () => ({
  staffService: {
    getAllStaff: mocks.getAllStaff,
  },
}));

vi.mock("~/lib/shift-type-api", () => ({
  shiftTypeService: {
    getShiftTypes: mocks.getShiftTypes,
  },
}));

import { useDienstplanData } from "./use-dienstplan-data";
import type { StaffScheduleOverview, StaffShift } from "~/lib/shift-helpers";
import type { Staff } from "~/lib/staff-api";
import type { ShiftType } from "~/lib/shift-type-helpers";

interface FakeSWRResult<T> {
  data: T | undefined;
  error: Error | undefined;
  isLoading: boolean;
  isValidating: boolean;
  mutate: () => Promise<unknown>;
}

function swrResult<T>(
  data: T | undefined,
  overrides: {
    error?: Error;
    isLoading?: boolean;
    mutate?: () => Promise<unknown>;
  } = {},
): FakeSWRResult<T> {
  return {
    data,
    error: overrides.error,
    isLoading: overrides.isLoading ?? false,
    isValidating: false,
    mutate: overrides.mutate ?? vi.fn().mockResolvedValue(undefined),
  };
}

describe("useDienstplanData", () => {
  const weekFrom = "2026-07-06";
  const weekTo = "2026-07-10";

  const overviewData: StaffScheduleOverview = {
    from: weekFrom,
    to: weekTo,
    dienstplanInUse: true,
    dienstplanUsedWeeks: [weekFrom],
    staff: [
      { id: "7", firstName: "Ada", lastName: "Lovelace" },
      { id: "3", firstName: "Bob", lastName: "Abel" },
    ],
    shifts: [
      {
        id: "101",
        staffId: "7",
        date: weekFrom,
        startTime: "08:00",
        endTime: "12:00",
        breakMinutes: 0,
        shiftTypeId: "1",
        shiftTypeName: "Betreuung",
        shiftTypeColor: "#83CD2D",
        notes: "",
        seriesId: null,
        detached: false,
        cancelled: false,
        changeReason: null,
        originShiftId: null,
      },
    ],
    assignments: [
      {
        instanceId: "91",
        staffId: "7",
        date: weekFrom,
        startTime: "12:00",
        endTime: "13:00",
        activityTitle: "Mensa",
        roomId: "4",
        roomName: "Speiseraum",
        status: "planned",
        isAbsent: false,
        isSubstitute: false,
        absenceReason: null,
        coverageStatus: "covered",
        coverageReason: null,
        uncoveredIntervals: [],
      },
    ],
    weeklySummaries: [
      {
        staffId: "7",
        weekStart: weekFrom,
        plannedMinutes: 240,
        targetMinutes: 300,
        deltaMinutes: -60,
      },
    ],
  };

  const legacyStaffData: Staff[] = [
    {
      id: "7",
      name: "Ada Lovelace",
      firstName: "Ada",
      lastName: "Lovelace",
      hasRfid: false,
      isTeacher: false,
      isSupervising: false,
      supervisions: [],
    },
  ];

  const legacyShiftsData: StaffShift[] = [
    {
      id: "202",
      staffId: "7",
      date: weekFrom,
      startTime: "08:00",
      endTime: "12:00",
      breakMinutes: 0,
      shiftTypeId: null,
      shiftTypeName: null,
      shiftTypeColor: null,
      notes: "",
      seriesId: null,
      detached: false,
      cancelled: false,
      changeReason: null,
      originShiftId: null,
    },
  ];

  const shiftTypesData: ShiftType[] = [
    {
      id: "1",
      name: "Betreuung",
      color: "#83CD2D",
      description: "",
      isActive: true,
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    mocks.useSession.mockReturnValue({
      data: { user: { id: "1", token: "token" } },
      status: "authenticated",
    });
    mocks.useTenantMutateMatching.mockReturnValue(
      vi.fn().mockResolvedValue(undefined),
    );
  });

  it("uses the full overview path when time_tracking:manage + schedules:read + users:read are granted", () => {
    mocks.hasPermission.mockReturnValue(true);
    const mutateOverview = vi.fn().mockResolvedValue(undefined);
    mocks.useSWRAuth.mockImplementation(
      (key: string | null, fetcher: () => Promise<unknown>) => {
        if (key === null) return swrResult(undefined);
        void fetcher();
        if (key.startsWith("dienstplan-overview-")) {
          return swrResult(overviewData, { mutate: mutateOverview });
        }
        if (key === "dienstplan-shift-types") {
          return swrResult(shiftTypesData);
        }
        return swrResult(undefined);
      },
    );

    const { result } = renderHook(() => useDienstplanData(weekFrom, weekTo));

    // Overview path used, single-fetch legacy path not used.
    expect(mocks.getOverview).toHaveBeenCalledWith(weekFrom, weekTo);
    expect(mocks.getAllStaff).not.toHaveBeenCalled();
    expect(mocks.getShifts).not.toHaveBeenCalled();

    expect(result.current.canManageAbsences).toBe(true);
    // Full permission set → not the reduced path.
    expect(result.current.reducedPath).toBe(false);
    // Sorted by last name, German collation: Abel before Lovelace.
    expect(result.current.sortedStaff.map((s) => s.id)).toEqual(["3", "7"]);
    expect(result.current.allShifts).toEqual(overviewData.shifts);
    expect(result.current.shiftsByStaff.get("7")?.get(weekFrom)).toEqual(
      overviewData.shifts,
    );
    expect(result.current.assignmentsByStaff.get("7")?.get(weekFrom)).toEqual(
      overviewData.assignments,
    );
    expect(result.current.summaryByStaff.get("7")).toEqual(
      overviewData.weeklySummaries[0],
    );
    expect(result.current.typesById.get("1")?.name).toBe("Betreuung");
    expect(result.current.scheduleError).toBeUndefined();
    expect(result.current.scheduleLoading).toBe(false);

    result.current.retryLoad();
    expect(mutateOverview).toHaveBeenCalledTimes(1);
  });

  it("uses the reduced staff/shifts path when only time_tracking:manage is granted", () => {
    mocks.hasPermission.mockImplementation(
      (_session: unknown, permission: string) =>
        permission === "time_tracking:manage",
    );
    const mutateLegacyStaff = vi.fn().mockResolvedValue(undefined);
    const mutateLegacyShifts = vi.fn().mockResolvedValue(undefined);
    mocks.useSWRAuth.mockImplementation(
      (key: string | null, fetcher: () => Promise<unknown>) => {
        if (key === null) return swrResult(undefined);
        void fetcher();
        if (key === "dienstplan-staff") {
          return swrResult(legacyStaffData, { mutate: mutateLegacyStaff });
        }
        if (key.startsWith("dienstplan-shifts-")) {
          return swrResult(legacyShiftsData, { mutate: mutateLegacyShifts });
        }
        if (key === "dienstplan-shift-types") {
          return swrResult(shiftTypesData);
        }
        return swrResult(undefined);
      },
    );

    const { result } = renderHook(() => useDienstplanData(weekFrom, weekTo));

    // Legacy path used, overview not used.
    expect(mocks.getAllStaff).toHaveBeenCalledTimes(1);
    expect(mocks.getShifts).toHaveBeenCalledWith(weekFrom, weekTo);
    expect(mocks.getOverview).not.toHaveBeenCalled();

    expect(result.current.canManageAbsences).toBe(true);
    // Missing schedules:read/users:read → the reduced path.
    expect(result.current.reducedPath).toBe(true);
    expect(result.current.sortedStaff).toEqual([
      { id: "7", firstName: "Ada", lastName: "Lovelace" },
    ]);
    expect(result.current.allShifts).toEqual(legacyShiftsData);
    // No assignments/summaries on the reduced path (no overview access).
    expect(result.current.assignmentsByStaff.size).toBe(0);
    expect(result.current.summaryByStaff.size).toBe(0);

    result.current.retryLoad();
    expect(mutateLegacyStaff).toHaveBeenCalledTimes(1);
    expect(mutateLegacyShifts).toHaveBeenCalledTimes(1);
  });

  it("surfaces the overview error as scheduleError on the full permission path", () => {
    mocks.hasPermission.mockReturnValue(true);
    const overviewError = new Error("server down");
    mocks.useSWRAuth.mockImplementation((key: string | null) => {
      if (key?.startsWith("dienstplan-overview-")) {
        return swrResult<StaffScheduleOverview>(undefined, {
          error: overviewError,
        });
      }
      return swrResult(undefined);
    });

    const { result } = renderHook(() => useDienstplanData(weekFrom, weekTo));

    expect(result.current.scheduleError).toBe(overviewError);
  });

  it("falls back to the shifts error when only the legacy shifts fetch fails", () => {
    mocks.hasPermission.mockImplementation(
      (_session: unknown, permission: string) =>
        permission === "time_tracking:manage",
    );
    const shiftsError = new Error("shifts down");
    mocks.useSWRAuth.mockImplementation((key: string | null) => {
      if (key === "dienstplan-staff") {
        return swrResult(legacyStaffData);
      }
      if (typeof key === "string" && key.startsWith("dienstplan-shifts-")) {
        return swrResult<StaffShift[]>(undefined, { error: shiftsError });
      }
      return swrResult(undefined);
    });

    const { result } = renderHook(() => useDienstplanData(weekFrom, weekTo));

    expect(result.current.scheduleError).toBe(shiftsError);
  });

  it("wires refreshPlanCaches to the Dienstplan and Vertretung cache prefixes (#1843)", () => {
    mocks.hasPermission.mockReturnValue(true);
    mocks.useSWRAuth.mockReturnValue(swrResult(undefined));

    renderHook(() => useDienstplanData(weekFrom, weekTo));

    expect(mocks.useTenantMutateMatching).toHaveBeenCalledWith([
      "dienstplan-overview-",
      "dienstplan-shifts-",
      "vertretung-week-",
      "vertretung-gaps-",
      "staff-pending-absences-",
      "time-tracking-absences-",
      "time-tracking-table-absences-",
      "time-tracking-own-absences-",
      "staff-shifts-visible-",
      "time-tracking-own-shifts-today-",
    ]);
  });
});
