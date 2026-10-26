/**
 * A roster action that saved, followed by a list reload that failed: the
 * page stays loaded, the failure shows on the shared error path, and
 * „Liste neu laden“ loads the list again (#5001).
 */
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const showActionError = vi.hoisted(() => vi.fn());
const errorDisplay = vi.hoisted(() => ({ show: showActionError }));
const formError = vi.hoisted(() => ({
  error: null,
  show: vi.fn(),
  clear: vi.fn(),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useApiErrorDisplay: () => errorDisplay,
  useApiFormError: () => formError,
}));

vi.mock("~/lib/supervision-context", () => ({
  useOptionalSupervision: () => ({ refresh: vi.fn() }),
}));

vi.mock("~/lib/timetable-operations-api", () => ({
  timetableOperationsApi: {
    patchAttendance: vi.fn(),
  },
  isReopenUnavailableError: vi.fn(() => false),
}));

import { ApiError } from "~/lib/api-error";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import type {
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";
import { useTimetableActions } from "./use-timetable-actions";

const row = {
  studentId: "7",
  studentName: "Mia Beispiel",
} as TimetableRosterRow;

function setup(mutateRoster: () => Promise<unknown>) {
  const options = {
    allRooms: [],
    currentStaffId: "1",
    activeTimetableInstanceId: "99",
    currentTimetableRoster: null as TimetableRoster | null,
    mutateRoster: vi.fn(mutateRoster),
    mutateDashboard: vi.fn(() => Promise.resolve(undefined)),
    adoptSession: vi.fn(() => "/active-supervisions"),
    setSelectedTimetableInstanceId: vi.fn(),
    router: { push: vi.fn() },
    reopenable: null,
    rememberReopenable: vi.fn(),
    clearReopenable: vi.fn(),
  };
  const hook = renderHook(() => useTimetableActions(options));
  return { ...hook, options };
}

describe("useTimetableActions roster sync after a saved action", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(timetableOperationsApi.patchAttendance).mockResolvedValue(
      undefined as never,
    );
  });

  it("shows the failed list reload with a retry instead of reloading the page", async () => {
    const failure = new ApiError("roster", 503, {
      code: "general.unavailable",
    });
    const { result, options } = setup(() => Promise.reject(failure));

    await act(async () => {
      await result.current.handleRosterAction("excused", row);
    });

    expect(timetableOperationsApi.patchAttendance).toHaveBeenCalledOnce();
    expect(options.mutateRoster).toHaveBeenCalledTimes(1);
    expect(showActionError).toHaveBeenCalledOnce();
    const [shownError, shownOptions] = showActionError.mock.calls[0]!;
    expect(shownError).toBe(failure);
    expect(shownOptions).toMatchObject({
      object: "die Anwesenheitsliste",
      messageSuffix: "Ihre Änderung ist gespeichert.",
      action: { label: "Liste neu laden" },
    });

    // „Liste neu laden“ loads the list again; it does not repeat the action.
    options.mutateRoster.mockResolvedValueOnce(undefined);
    await act(async () => {
      (shownOptions as { action: { onClick: () => void } }).action.onClick();
      await Promise.resolve();
    });

    expect(options.mutateRoster).toHaveBeenCalledTimes(2);
    expect(timetableOperationsApi.patchAttendance).toHaveBeenCalledOnce();
    expect(showActionError).toHaveBeenCalledOnce();
  });

  it("offers to load the list again when that fails too", async () => {
    const failure = new ApiError("roster", 503, {
      code: "general.unavailable",
    });
    const { result, options } = setup(() => Promise.reject(failure));

    await act(async () => {
      await result.current.handleRosterAction("absent", row);
    });
    const [, firstOptions] = showActionError.mock.calls[0]!;

    await act(async () => {
      (firstOptions as { action: { onClick: () => void } }).action.onClick();
      await Promise.resolve();
    });

    expect(options.mutateRoster).toHaveBeenCalledTimes(2);
    expect(showActionError).toHaveBeenCalledTimes(2);
    expect(showActionError.mock.calls[1]![1]).toMatchObject({
      object: "die Anwesenheitsliste",
      messageSuffix: "Ihre Änderung ist gespeichert.",
      action: { label: "Liste neu laden" },
    });
  });
});
