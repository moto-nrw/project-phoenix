/**
 * The present-children side of useTimetableActions (#3824): a spontaneous
 * start asks for the picker once, and a selection goes to the server as one
 * bulk check-in.
 */
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const refreshSupervision = vi.hoisted(() => vi.fn());

vi.mock("~/lib/supervision-context", () => ({
  useOptionalSupervision: () => ({ refresh: refreshSupervision }),
}));

vi.mock("~/lib/timetable-operations-api", () => ({
  timetableOperationsApi: {
    createAndStartSpontaneous: vi.fn(),
    checkInMany: vi.fn(),
  },
  isReopenUnavailableError: vi.fn(() => false),
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import type { TimetableRoster } from "~/lib/timetable-operations-types";
import { useTimetableActions } from "./use-timetable-actions";

const roster = {
  instance: {
    id: "99",
    title: "Fußball-AG",
    status: "active",
    isSpontaneous: true,
    activeGroupId: "4",
    roomId: "20",
    date: "2026-09-09",
    startTime: "12:00",
    endTime: "13:00",
    canComplete: false,
    completeAvailableAt: "",
  },
  rows: [],
} as TimetableRoster;

function setup() {
  const options = {
    allRooms: [],
    currentStaffId: "1",
    activeTimetableInstanceId: "99",
    currentTimetableRoster: roster,
    mutateRoster: vi.fn(() => Promise.resolve(undefined)),
    mutateDashboard: vi.fn(() => Promise.resolve(undefined)),
    adoptSession: vi.fn(
      (activeGroupId: string) =>
        `/active-supervisions?session=${activeGroupId}`,
    ),
    setSelectedTimetableInstanceId: vi.fn(),
    router: { push: vi.fn() },
    reopenable: null,
    rememberReopenable: vi.fn(),
    clearReopenable: vi.fn(),
  };
  const hook = renderHook(() => useTimetableActions(options), {
    wrapper: ToastProvider,
  });
  return { ...hook, options };
}

describe("useTimetableActions present children (#3824)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    refreshSupervision.mockResolvedValue(undefined);
  });

  it("asks once for the picker of a spontaneous activity it just started", async () => {
    vi.mocked(
      timetableOperationsApi.createAndStartSpontaneous,
    ).mockResolvedValue({
      instanceId: "99",
      activeGroupId: "4",
      status: "active",
    });
    const { result } = setup();
    expect(result.current.presentPickerAutoOpenInstanceId).toBeNull();

    await act(async () => {
      await result.current.handleStartSpontaneousActivity({
        title: "Fußball-AG",
        roomId: "20",
        additionalStaffIds: [],
      });
    });

    expect(result.current.presentPickerAutoOpenInstanceId).toBe("99");
    act(() => result.current.clearPresentPickerAutoOpen());
    expect(result.current.presentPickerAutoOpenInstanceId).toBeNull();
  });

  it("checks the selection in with one request and shows the new roster", async () => {
    const updated = { ...roster, rows: [] } as TimetableRoster;
    vi.mocked(timetableOperationsApi.checkInMany).mockResolvedValue(updated);
    const { result, options } = setup();

    let added = false;
    await act(async () => {
      added = await result.current.handleAddPresentStudents(["5", "6"]);
    });

    expect(added).toBe(true);
    expect(timetableOperationsApi.checkInMany).toHaveBeenCalledWith("99", [
      "5",
      "6",
    ]);
    expect(options.mutateRoster).toHaveBeenCalledWith(updated, {
      revalidate: false,
    });
    expect(result.current.isAddingStudent).toBe(false);
  });

  it("keeps the failure in the dialog and reports nothing added", async () => {
    vi.mocked(timetableOperationsApi.checkInMany).mockRejectedValue(
      new ApiError("conflict", 409, { code: "timetable.operation_stale" }),
    );
    const { result, options } = setup();

    let added = true;
    await act(async () => {
      added = await result.current.handleAddPresentStudents(["5"]);
    });

    expect(added).toBe(false);
    expect(options.mutateRoster).not.toHaveBeenCalled();
    await waitFor(() => expect(result.current.addStudentError).not.toBeNull());
  });
});
