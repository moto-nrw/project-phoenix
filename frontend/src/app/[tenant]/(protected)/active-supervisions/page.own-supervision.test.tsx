/**
 * „Aktuelle Aufsicht“ ohne groups:read (#4573, AG-Leitung): die Seite
 * zeigt nur die eigene laufende Aufsicht aus der Betriebs-API des
 * Betreuungsplans, ohne die schulweite Übersicht. Wer groups:read hält,
 * behält die volle Seite.
 */
import {
  cleanup,
  fireEvent,
  render as rtlRender,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const state = vi.hoisted(() => ({
  mobile: false,
  permissions: [] as string[],
  sessionParam: null as string | null,
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { token: "test-token", permissions: state.permissions } },
    status: "authenticated",
  }),
}));

vi.mock("~/lib/hooks/use-media-query", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/hooks/use-media-query")>()),
  useMediaQuery: () => state.mobile,
}));

const mockPush = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
  useSearchParams: () => ({
    get: (key: string) => (key === "session" ? state.sessionParam : null),
  }),
  redirect: vi.fn(),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: vi.fn(() => ({ breadcrumb: {}, setBreadcrumb: vi.fn() })),
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

vi.mock("~/components/ui/modal", () => ({
  dialogAriaProps: { role: "dialog" as const, "aria-modal": true },
  Modal: ({
    isOpen,
    children,
  }: {
    isOpen: boolean;
    children: React.ReactNode;
  }) => (isOpen ? <div>{children}</div> : null),
  ConfirmationModal: ({
    isOpen,
    children,
    confirmText,
    onConfirm,
  }: {
    isOpen: boolean;
    children: React.ReactNode;
    confirmText: string;
    onConfirm: () => void;
  }) =>
    isOpen ? (
      <div>
        {children}
        <button type="button" onClick={onConfirm}>
          {confirmText}
        </button>
      </div>
    ) : null,
}));

vi.mock("~/components/sse/SSEErrorBoundary", () => ({
  SSEErrorBoundary: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

// Die volle Seite: nur, ob sie ihre schulweite Übersicht anfragt.
const dashboardHook = vi.hoisted(() => vi.fn());
vi.mock("~/components/active-supervisions/use-supervision-dashboard", () => ({
  useSupervisionDashboard: dashboardHook,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  useTenantMutate: vi.fn(() => vi.fn()),
}));

vi.mock("~/lib/timetable-operations-api", () => ({
  timetableOperationsApi: {
    plannedNow: vi.fn(),
    roster: vi.fn(),
    checkIn: vi.fn(),
    checkOut: vi.fn(),
    complete: vi.fn(),
    patchAttendance: vi.fn(),
    reopen: vi.fn(),
    start: vi.fn(),
  },
  isReopenUnavailableError: vi.fn(() => false),
}));

import { ApiError } from "~/lib/api-error";
import { useSWRAuth } from "~/lib/swr";
import {
  useAttendanceWebEnabled,
  useShowTimetableCounts,
} from "~/lib/tenant-context";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import { ToastProvider } from "~/contexts/ToastContext";
import MeinRaumPage from "./page";

function render(ui: Parameters<typeof rtlRender>[0]) {
  return rtlRender(ui, { wrapper: ToastProvider });
}

const runningBlock = {
  id: "99",
  title: "AG Musik",
  date: "2026-09-09",
  startTime: "11:30",
  endTime: "13:00",
  roomId: "10",
  roomName: "Musikraum",
  status: "active",
  isOverdue: false,
  minutesUntilStart: 0,
  expectedStudentsCount: 1,
  presentStudentsCount: 0,
  notScheduledStudentsCount: 0,
  assignedStaffIds: ["7"],
  isAssigned: true,
  isPrimary: true,
  isSubstitute: false,
  isAbsent: false,
  rosterPreview: [],
  activeGroupId: "500",
};

const otherRunningBlock = {
  ...runningBlock,
  id: "98",
  title: "Fremder Block",
  isAssigned: false,
  activeGroupId: "501",
};

const expectedRow = {
  studentId: "100",
  studentName: "Marie Muster",
  schoolClass: "2b",
  groupName: "OGS Gruppe A",
  planned: true,
  isUnplanned: false,
  currentlyPresent: false,
  visitId: null,
  status: "expected",
  substatus: null,
  note: null,
  checkedInAt: null,
  visitEntryTime: null,
  warnings: [],
  careDayStatus: "scheduled",
};

const roster = {
  instance: {
    id: "99",
    title: "AG Musik",
    status: "active",
    isSpontaneous: false,
    activeGroupId: "500",
    roomId: "10",
    date: "2026-09-09",
    startTime: "11:30",
    endTime: "13:00",
    canComplete: true,
    completeAvailableAt: "",
  },
  rows: [expectedRow],
};

const swrKeys: (string | null)[] = [];
const mockMutate = vi.fn();

function swr(data: unknown) {
  return {
    data,
    isLoading: false,
    error: undefined,
    mutate: mockMutate,
    isValidating: false,
  };
}

describe("MeinRaumPage without groups:read (#4573)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    swrKeys.length = 0;
    state.mobile = false;
    state.permissions = ["schedules:read"];
    state.sessionParam = "500";
    localStorage.clear();
    sessionStorage.clear();
    mockMutate.mockResolvedValue(undefined);
    vi.mocked(useAttendanceWebEnabled).mockReturnValue(true);
    vi.mocked(useShowTimetableCounts).mockReturnValue(true);
    global.fetch = vi.fn();
    dashboardHook.mockImplementation(() => {
      throw new Error("the school-wide dashboard must not load");
    });
    vi.mocked(useSWRAuth).mockImplementation(((key: string | null) => {
      swrKeys.push(key);
      if (key === "active-supervisions-own-sessions") return swr([]);
      if (key === "active-supervisions-own-day") {
        return swr([runningBlock, otherRunningBlock]);
      }
      if (key === "active-supervisions-own-planned-now") return swr([]);
      if (key === "timetable-roster-99") return swr(roster);
      return swr(undefined);
    }) as never);
    vi.mocked(timetableOperationsApi.checkIn).mockResolvedValue({
      ...roster,
      rows: [
        {
          ...expectedRow,
          currentlyPresent: true,
          visitId: "visit-100",
          status: "present",
        },
      ],
    } as never);
    vi.mocked(timetableOperationsApi.complete).mockResolvedValue({
      reopenUntil: null,
    } as never);
  });

  afterEach(() => {
    cleanup();
  });

  it("shows the own running block with its roster and no school-wide request", async () => {
    render(<MeinRaumPage />);

    expect(await screen.findByText("Marie Muster")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 1, name: "AG Musik" }),
    ).toBeInTheDocument();
    expect(dashboardHook).not.toHaveBeenCalled();
    expect(
      swrKeys.some((key) => key?.startsWith("active-supervision-dashboard")),
    ).toBe(false);
    expect(global.fetch).not.toHaveBeenCalled();
    expect(screen.queryByText("Fremder Block")).not.toBeInTheDocument();
  });

  it("shows an initial roster failure and retries all four requests", async () => {
    const rosterError = new ApiError("roster unavailable", 503);
    const mutateDay = vi.fn().mockResolvedValue(undefined);
    const mutateUpcoming = vi.fn().mockResolvedValue(undefined);
    const mutateOwnSessions = vi.fn().mockResolvedValue(undefined);
    const mutateRoster = vi.fn().mockResolvedValue(undefined);
    vi.mocked(useSWRAuth).mockImplementation(((key: string | null) => {
      if (key === "active-supervisions-own-sessions")
        return { ...swr([]), mutate: mutateOwnSessions };
      if (key === "active-supervisions-own-day") {
        return { ...swr([runningBlock]), mutate: mutateDay };
      }
      if (key === "active-supervisions-own-planned-now") {
        return { ...swr([]), mutate: mutateUpcoming };
      }
      if (key === "timetable-roster-99") {
        return { ...swr(undefined), error: rosterError, mutate: mutateRoster };
      }
      return swr(undefined);
    }) as never);

    const view = render(<MeinRaumPage />);

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => {
      expect(mutateDay).toHaveBeenCalledOnce();
      expect(mutateUpcoming).toHaveBeenCalledOnce();
      expect(mutateRoster).toHaveBeenCalledOnce();
      expect(mutateOwnSessions).toHaveBeenCalledOnce();
    });

    vi.mocked(useSWRAuth).mockImplementation(((key: string | null) => {
      if (key === "active-supervisions-own-sessions") return swr([]);
      if (key === "active-supervisions-own-day") return swr([runningBlock]);
      if (key === "active-supervisions-own-planned-now") return swr([]);
      return swr(key === "timetable-roster-99" ? roster : undefined);
    }) as never);
    view.rerender(<MeinRaumPage />);
    expect(await screen.findByText("Marie Muster")).toBeInTheDocument();
  });

  it("checks a child in through the timetable operations route", async () => {
    render(<MeinRaumPage />);

    fireEvent.click(await screen.findByRole("button", { name: "Einchecken" }));

    await waitFor(() =>
      expect(timetableOperationsApi.checkIn).toHaveBeenCalledWith("99", "100"),
    );
  });

  it("completes the block through the timetable operations route", async () => {
    render(<MeinRaumPage />);

    fireEvent.click(await screen.findByRole("button", { name: "Beenden" }));
    fireEvent.click(
      await screen.findByRole("button", { name: "Aktivität beenden" }),
    );

    await waitFor(() =>
      expect(timetableOperationsApi.complete).toHaveBeenCalledWith("99", []),
    );
  });

  it("opens an additional supervision from the sidebar without a planned assignment", async () => {
    state.sessionParam = "501";
    vi.mocked(useSWRAuth).mockImplementation(((key: string | null) => {
      if (key === "active-supervisions-own-day") return swr([]);
      if (key === "active-supervisions-own-planned-now") return swr([]);
      if (key === "active-supervisions-own-sessions") {
        return swr([{ id: "501", name: "AG Musik" }]);
      }
      if (key === "timetable-roster-active-group-501") {
        return swr({
          ...roster,
          instance: { ...roster.instance, activeGroupId: "501" },
        });
      }
      return swr(undefined);
    }) as never);

    render(<MeinRaumPage />);

    expect(await screen.findByText("Marie Muster")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Einchecken" }));
    await waitFor(() =>
      expect(timetableOperationsApi.checkIn).toHaveBeenCalledWith("99", "100"),
    );
  });

  it("includes additional supervisions once and switches between them on mobile", async () => {
    state.mobile = true;
    state.sessionParam = "501";
    vi.mocked(useSWRAuth).mockImplementation(((key: string | null) => {
      if (key === "active-supervisions-own-day")
        return swr([runningBlock, otherRunningBlock]);
      if (key === "active-supervisions-own-planned-now") return swr([]);
      if (key === "active-supervisions-own-sessions")
        return swr([
          { id: "500", name: "Aufsicht" },
          { id: "501", name: "Aufsicht" },
        ]);
      if (key === "timetable-roster-98")
        return swr({
          ...roster,
          instance: {
            ...roster.instance,
            id: "98",
            title: "Fremder Block",
            activeGroupId: "501",
          },
        });
      return swr(undefined);
    }) as never);

    render(<MeinRaumPage />);

    expect(await screen.findByText("Marie Muster")).toBeInTheDocument();
    expect(screen.getAllByRole("tab")).toHaveLength(2);
    fireEvent.click(screen.getByRole("tab", { name: "AG Musik" }));
    expect(mockPush).toHaveBeenCalledWith(
      "/test-tenant/active-supervisions?session=500",
    );
  });

  it("does not offer an additional supervision no longer returned by the own endpoint", async () => {
    vi.mocked(useSWRAuth).mockImplementation(((key: string | null) => {
      swrKeys.push(key);
      if (key === "active-supervisions-own-day")
        return swr([otherRunningBlock]);
      if (key === "active-supervisions-own-planned-now") return swr([]);
      if (key === "active-supervisions-own-sessions") return swr([]);
      return swr(undefined);
    }) as never);

    render(<MeinRaumPage />);

    expect(
      await screen.findByText("Keine laufende Aufsicht"),
    ).toBeInTheDocument();
    expect(swrKeys.some((key) => key?.startsWith("timetable-roster-"))).toBe(
      false,
    );
  });

  it("reports a failed own-supervision request and retries it", async () => {
    const retry = vi.fn().mockResolvedValue(undefined);
    vi.mocked(useSWRAuth).mockImplementation(((key: string | null) => {
      if (key === "active-supervisions-own-day") return swr([runningBlock]);
      if (key === "active-supervisions-own-planned-now") return swr([]);
      if (key === "active-supervisions-own-sessions")
        return {
          ...swr(undefined),
          error: new ApiError("unavailable", 503),
          mutate: retry,
        };
      return swr(key === "timetable-roster-99" ? roster : undefined);
    }) as never);

    render(<MeinRaumPage />);

    expect(await screen.findByText("Marie Muster")).toBeInTheDocument();
    expect(screen.getByRole("alert")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(retry).toHaveBeenCalledOnce());
  });

  it("names the missing supervision when no own block runs", async () => {
    vi.mocked(useSWRAuth).mockImplementation(((key: string | null) => {
      swrKeys.push(key);
      if (key === "active-supervisions-own-sessions") return swr([]);
      if (key === "active-supervisions-own-day") {
        return swr([otherRunningBlock]);
      }
      return swr(
        key === "active-supervisions-own-planned-now" ? [] : undefined,
      );
    }) as never);

    render(<MeinRaumPage />);

    expect(
      await screen.findByText("Keine laufende Aufsicht"),
    ).toBeInTheDocument();
    expect(swrKeys.some((key) => key?.startsWith("timetable-roster-"))).toBe(
      false,
    );
  });

  it("shows the forbidden state without schedules:read", async () => {
    state.permissions = [];

    render(<MeinRaumPage />);

    expect(
      await screen.findByText(/Ihnen fehlt eine Berechtigung/),
    ).toBeInTheDocument();
    expect(dashboardHook).not.toHaveBeenCalled();
  });

  it("keeps the full page for staff with groups:read", () => {
    state.permissions = ["groups:read", "schedules:read"];
    dashboardHook.mockImplementation(() => {
      throw new Error("full page");
    });

    expect(() => render(<MeinRaumPage />)).toThrow("full page");
    expect(dashboardHook).toHaveBeenCalled();
    expect(swrKeys).not.toContain("active-supervisions-own-day");
  });
});
