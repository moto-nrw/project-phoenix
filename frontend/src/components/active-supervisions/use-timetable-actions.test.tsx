/**
 * The lifecycle side of useTimetableActions: after an own start, end or undo
 * the page and the sidebar reload together, and the end dialog stays pending
 * until they have (#3888).
 */
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// The sidebar memory and the offering choice belong to the signed-in account
// (#5000), so the hooks read the session.
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "7", tenantId: 1 } },
    status: "authenticated",
  }),
}));

const refreshSupervision = vi.hoisted(() => vi.fn());

vi.mock("~/lib/supervision-context", () => ({
  useOptionalSupervision: () => ({ refresh: refreshSupervision }),
}));

vi.mock("~/lib/timetable-operations-api", () => ({
  timetableOperationsApi: {
    complete: vi.fn(),
    createAndStartSpontaneous: vi.fn(),
    reopen: vi.fn(),
    start: vi.fn(),
  },
  isReopenUnavailableError: vi.fn(() => false),
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import type { TimetableRoster } from "~/lib/timetable-operations-types";
import {
  bindPersonalStorage,
  clearPersonalStorage,
} from "~/lib/personal-storage";
import { sidebarMemoryKey } from "~/lib/sidebar-memory";
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
    // The undo belongs to the signed-in account (#5000).
    bindPersonalStorage("1:7");
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

  it("keeps a selected external caregiver's large staff ID as a decimal string", async () => {
    vi.mocked(
      timetableOperationsApi.createAndStartSpontaneous,
    ).mockResolvedValue({
      instanceId: "88",
      activeGroupId: "5",
      status: "active",
    });
    const { result } = setup();

    await act(async () => {
      await result.current.handleStartSpontaneousActivity({
        title: "Musik-AG",
        roomId: "20",
        additionalStaffIds: ["9007199254740993"],
      });
    });

    expect(
      timetableOperationsApi.createAndStartSpontaneous,
    ).toHaveBeenCalledWith({
      title: "Musik-AG",
      room_id: 20,
      activity_group_id: undefined,
      staff_ids: ["1", "9007199254740993"],
    });
  });
});

// The request starts for account 7 at school 1, the tab then switches to
// another account or logs out, and only then the answer arrives (#5000). The
// sidebar memory and the undo banner must not bring back what the switch
// cleared: the next person would see the previous person's room and activity.
describe("useTimetableActions after a logout or account switch (#5000)", () => {
  const LAST_ROOM = sidebarMemoryKey("sidebar-last-room", "1:7");
  const LAST_SESSION = sidebarMemoryKey("supervision-last-session", "1:7");

  beforeEach(() => {
    vi.clearAllMocks();
    refreshSupervision.mockResolvedValue(undefined);
    localStorage.clear();
    bindPersonalStorage("1:7");
  });

  afterEach(() => {
    localStorage.clear();
  });

  const changes = [
    ["an account switch", () => bindPersonalStorage("1:8")],
    ["a logout", () => clearPersonalStorage()],
  ] as const;

  it("remembers the started block while the account stays", async () => {
    vi.mocked(timetableOperationsApi.start).mockResolvedValue({
      instanceId: "88",
      activeGroupId: "5",
      status: "active",
    });
    const { result } = setup();

    await act(async () => {
      await result.current.handleStartPlannedInstance({
        id: "88",
        roomId: "20",
      } as Parameters<typeof result.current.handleStartPlannedInstance>[0]);
    });

    expect(localStorage.getItem(LAST_ROOM)).toBe("20");
    expect(localStorage.getItem(LAST_SESSION)).toBe("5");
  });

  it.each(changes)(
    "does not remember a block whose start answers after %s",
    async (_change, change) => {
      let answer!: (value: {
        instanceId: string;
        activeGroupId: string;
        status: string;
      }) => void;
      vi.mocked(timetableOperationsApi.start).mockReturnValue(
        new Promise((resolve) => {
          answer = resolve;
        }),
      );
      const { result, options } = setup();

      let started!: Promise<void>;
      act(() => {
        started = result.current.handleStartPlannedInstance({
          id: "88",
          roomId: "20",
        } as Parameters<typeof result.current.handleStartPlannedInstance>[0]);
      });
      change();
      await act(async () => {
        answer({ instanceId: "88", activeGroupId: "5", status: "active" });
        await started;
      });

      // Anker: the start went through.
      expect(options.router.push).toHaveBeenCalledWith(
        "/active-supervisions?session=5",
      );
      expect(localStorage.getItem(LAST_ROOM)).toBeNull();
      expect(localStorage.getItem(LAST_SESSION)).toBeNull();
    },
  );

  it.each(changes)(
    "offers no undo for an end that answers after %s",
    async (_change, change) => {
      let answer!: (value: { reopenUntil: string }) => void;
      vi.mocked(timetableOperationsApi.complete).mockReturnValue(
        new Promise((resolve) => {
          answer = resolve;
        }),
      );
      const { result, options } = setup();

      let completed!: Promise<void>;
      act(() => {
        completed = result.current.confirmCompleteTimetableInstance();
      });
      change();
      await act(async () => {
        answer({ reopenUntil: "2026-09-09T12:05:00+02:00" });
        await completed;
      });

      // Anker: the end went through and the page reloaded.
      expect(refreshSupervision).toHaveBeenCalledWith({
        silent: true,
        force: true,
      });
      expect(options.rememberReopenable).not.toHaveBeenCalled();
    },
  );
});
