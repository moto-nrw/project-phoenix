/**
 * The lifecycle side of useTimetableActions: after an own start, end or undo
 * the page and the sidebar reload together, and the end dialog stays pending
 * until they have (#3888).
 */
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const refreshSupervision = vi.hoisted(() => vi.fn());

vi.mock("~/lib/supervision-context", () => ({
  useOptionalSupervision: () => ({ refresh: refreshSupervision }),
}));

vi.mock("~/lib/timetable-operations-api", () => ({
  timetableOperationsApi: {
    complete: vi.fn(),
    reopen: vi.fn(),
    start: vi.fn(),
  },
  isReopenUnavailableError: vi.fn(() => false),
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import type { TimetableRoster } from "~/lib/timetable-operations-types";
import { useTimetableActions } from "./use-timetable-actions";

function deferred() {
  let resolve: () => void = () => undefined;
  let reject: (err: Error) => void = () => undefined;
  const promise = new Promise<void>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

const roster = {
  instance: {
    id: "99",
    title: "Mittagessen",
    status: "active",
    isSpontaneous: false,
    activeGroupId: "4",
    roomId: "20",
    date: "2026-09-09",
    startTime: "12:00",
    endTime: "13:00",
    canComplete: true,
    completeAvailableAt: "",
  },
  rows: [],
} as TimetableRoster;

function setup(overrides: { mutateDashboard?: () => Promise<unknown> } = {}) {
  const options = {
    allRooms: [],
    currentStaffId: "1",
    activeTimetableInstanceId: "99",
    currentTimetableRoster: roster,
    mutateRoster: vi.fn(() => Promise.resolve(undefined)),
    mutateDashboard:
      overrides.mutateDashboard ?? vi.fn(() => Promise.resolve(undefined)),
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

describe("useTimetableActions lifecycle reload (#3888)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    refreshSupervision.mockResolvedValue(undefined);
    vi.mocked(timetableOperationsApi.complete).mockResolvedValue({
      reopenUntil: "2026-09-09T12:05:00+02:00",
    });
  });

  afterEach(() => {
    localStorage.clear();
  });

  it("keeps the end dialog pending until page and sidebar have reloaded", async () => {
    const dashboard = deferred();
    const { result, options } = setup({
      mutateDashboard: vi.fn(() => dashboard.promise),
    });

    act(() => result.current.setShowCompleteConfirmation(true));
    act(() => {
      void result.current.confirmCompleteTimetableInstance();
    });

    await waitFor(() =>
      expect(refreshSupervision).toHaveBeenCalledWith({
        silent: true,
        force: true,
      }),
    );
    expect(options.rememberReopenable).toHaveBeenCalledWith(
      { instanceId: "99", title: "Mittagessen", roomId: "20" },
      "2026-09-09T12:05:00+02:00",
    );
    // Still pending: the ended block must not reappear as running.
    expect(result.current.isCompletingInstance).toBe(true);
    expect(result.current.showCompleteConfirmation).toBe(true);
    expect(options.setSelectedTimetableInstanceId).not.toHaveBeenCalled();

    await act(async () => {
      dashboard.resolve();
      await dashboard.promise;
    });

    await waitFor(() =>
      expect(result.current.isCompletingInstance).toBe(false),
    );
    expect(result.current.showCompleteConfirmation).toBe(false);
    expect(options.setSelectedTimetableInstanceId).toHaveBeenCalledWith(null);
    // One aggregate load; a second one kept the ended block on screen.
    expect(options.mutateDashboard).toHaveBeenCalledTimes(1);
    expect(result.current.completeError).toBeNull();
  });

  it("closes the dialog without an error when only the reload fails", async () => {
    const { result } = setup({
      mutateDashboard: vi.fn(() => Promise.reject(new Error("offline"))),
    });

    act(() => result.current.setShowCompleteConfirmation(true));
    await act(async () => {
      await result.current.confirmCompleteTimetableInstance();
    });

    expect(result.current.showCompleteConfirmation).toBe(false);
    expect(result.current.completeError).toBeNull();
  });

  it("reloads the sidebar after starting a planned block", async () => {
    vi.mocked(timetableOperationsApi.start).mockResolvedValue({
      instanceId: "88",
      activeGroupId: "5",
      status: "active",
    });
    const { result, options } = setup();

    await act(async () => {
      await result.current.handleStartPlannedInstance({
        id: "88",
        roomId: "20",
      } as Parameters<typeof result.current.handleStartPlannedInstance>[0]);
    });

    expect(options.router.push).toHaveBeenCalledWith(
      "/active-supervisions?session=5",
    );
    expect(refreshSupervision).toHaveBeenCalledWith({
      silent: true,
      force: true,
    });
  });
});
