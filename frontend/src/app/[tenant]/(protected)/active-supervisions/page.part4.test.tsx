/**
 * Tests for Active Supervisions Page
 * Tests the rendering states and user interactions of the active supervisions dashboard
 *
 * NOTE: split into 11 files (page.test.tsx + page.part2..11.test.tsx). The full-dashboard
 * render tests in the "MeinRaumPage (Active Supervisions)" describe are memory-heavy under
 * happy-dom + v8 coverage (~1.5 GB heap each), so a single combined file OOMs the Vitest
 * worker. Those heavy tests are pre-split into (N/M) chunks of 3 renders each, one chunk per
 * file (a 3-render chunk fits comfortably in a 6 GB heap; CI runs with 8 GB). All other
 * describes render cheaply and are packed together. All files share the identical mock header
 * below. When adding a heavy full-dashboard render test, keep it to its own small file.
 */
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

const navigationMockState = vi.hoisted(() => ({
  roomParam: null as string | null,
}));

// Mock auth-utils with hasRole that reads session roles
vi.mock("~/lib/auth-utils", () => ({
  isAdmin: (session: { user?: { isAdmin?: boolean } } | null) =>
    session?.user?.isAdmin ?? false,
  isCaregiver: (session: { user?: { isAdmin?: boolean } } | null) =>
    !(session?.user?.isAdmin ?? false),
  hasRole: (session: { user?: { isAdmin?: boolean } } | null, role: string) => {
    if (role === "admin") return session?.user?.isAdmin ?? false;
    if (role === "user") return !(session?.user?.isAdmin ?? false);
    return false;
  },
}));

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { token: "test-token" } },
    status: "authenticated",
  })),
}));

// Mock next/navigation
const mockPush = vi.fn();
const mockRedirect = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
  useSearchParams: () => ({
    get: (key: string) =>
      key === "room" ? navigationMockState.roomParam : null,
  }),
  redirect: (url: string) => mockRedirect(url),
}));

// Mock breadcrumb context
vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: vi.fn(() => ({ breadcrumb: {}, setBreadcrumb: vi.fn() })),
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

// Mock Loading component
vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading">Loading...</div>,
}));

// Mock PageHeaderWithSearch (vi.fn wrapper enables mockImplementation in enhanced tests)
vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: vi.fn(
    ({ title, badge }: { title: string; badge?: { count: number } }) => (
      <div data-testid="page-header" data-count={badge?.count}>
        {title}
      </div>
    ),
  ),
}));

// Mock Alert
vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message, type }: { message: string; type: string }) => (
    <div data-testid={`alert-${type}`}>{message}</div>
  ),
}));

// Mock Modal and ConfirmationModal
vi.mock("~/components/ui/modal", () => ({
  dialogAriaProps: { role: "dialog", "aria-modal": true },
  Modal: ({
    isOpen,
    children,
    title,
  }: {
    isOpen: boolean;
    children: React.ReactNode;
    title: string;
  }) =>
    isOpen ? (
      <div data-testid="modal" data-title={title}>
        {children}
      </div>
    ) : null,
  ConfirmationModal: ({
    isOpen,
    children,
    title,
  }: {
    isOpen: boolean;
    children: React.ReactNode;
    title: string;
  }) =>
    isOpen ? (
      <div data-testid="confirmation-modal" data-title={title}>
        {children}
      </div>
    ) : null,
}));

// Mock activeService
vi.mock("~/lib/active-api", () => ({
  activeService: {
    getActiveGroupVisitsWithDisplay: vi.fn(() => Promise.resolve([])),
    getActiveGroupSupervisors: vi.fn(() => Promise.resolve([])),
    endSupervision: vi.fn(() => Promise.resolve()),
    toggleSchulhofSupervision: vi.fn(() => Promise.resolve()),
    getTrackingIndicators: vi.fn(() =>
      Promise.resolve({ labels: [], results: {} }),
    ),
  },
}));

vi.mock("~/lib/activity-service", () => ({
  activityService: {
    getActivities: vi.fn(() => Promise.resolve([])),
  },
}));

vi.mock("~/lib/staff-api", () => ({
  staffService: {
    getAllStaff: vi.fn(() => Promise.resolve([])),
  },
}));

// Mock SSEErrorBoundary
vi.mock("~/components/sse/SSEErrorBoundary", () => ({
  SSEErrorBoundary: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="sse-boundary">{children}</div>
  ),
}));

// Mock UnclaimedRooms
vi.mock("~/components/active/unclaimed-rooms", () => ({
  UnclaimedRooms: () => <div data-testid="unclaimed-rooms" />,
}));

// Mock LocationBadge
vi.mock("@/components/ui/location-badge", () => ({
  LocationBadge: () => <div data-testid="location-badge">Location</div>,
}));

// Mock EmptyStudentResults
vi.mock("~/components/ui/empty-student-results", () => ({
  EmptyStudentResults: () => <div data-testid="empty-results">No results</div>,
}));

// Mock location-helper
vi.mock("~/lib/location-helper", () => ({
  LOCATION_COLORS: {
    UNKNOWN: "#6B7280",
    SCHOOLYARD: "#F78C10",
    HOME: "#FF3130",
    GROUP_ROOM: "#83CD2D",
  },
  LOCATION_STATUSES: { PRESENT: "Anwesend" },
  isHomeLocation: vi.fn(() => false),
  isSchoolyardLocation: vi.fn(() => false),
  isTransitLocation: vi.fn(() => false),
  parseLocation: vi.fn(() => ({ room: "Room 1", status: "Anwesend" })),
}));

// Mock pickup-helpers
vi.mock("~/lib/pickup-helpers", () => ({
  useMinuteClock: () => new Date("2026-01-15T12:00:00"),
}));

// Mock pickup-schedule-api
vi.mock("~/lib/pickup-schedule-api", () => ({
  fetchBulkPickupTimes: vi.fn(() => Promise.resolve(new Map())),
}));

// Mock student-arrival-api
vi.mock("~/lib/student-arrival-api", () => ({
  fetchBulkArrivalTimes: vi.fn(() => Promise.resolve(new Map())),
}));

// Mock StudentCard components
vi.mock("~/components/students/student-card", () => ({
  StudentCard: ({
    firstName,
    lastName,
    extraContent,
  }: {
    firstName: string;
    lastName: string;
    extraContent?: React.ReactNode;
  }) => (
    <div data-testid="student-card">
      {firstName} {lastName}
      {extraContent}
    </div>
  ),
  StudentInfoRow: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="student-info-row">{children}</div>
  ),
  SchoolClassIcon: () => <span data-testid="school-class-icon" />,
  GroupIcon: () => <span data-testid="group-icon" />,
  PickupTimeRow: ({
    pickupTime,
    isException,
    notes,
    isHome,
  }: {
    pickupTime?: string;
    isException: boolean;
    notes?: string;
    isHome: boolean;
    now: Date;
  }) => (
    <div
      data-testid="pickup-time-row"
      data-pickup-time={pickupTime ?? ""}
      data-is-exception={String(isException)}
      data-is-home={String(isHome)}
    >
      {pickupTime && <>Abholzeit: {pickupTime} Uhr</>}
      {!pickupTime && isException && (notes || "Abwesend")}
      {!pickupTime && !isException && <>Abholzeit: —</>}
      {notes && <span>({notes})</span>}
    </div>
  ),
  ArrivalTimeRow: ({
    arrivalTime,
    isException,
    isAbsent,
    notes,
    isHome,
  }: {
    arrivalTime?: string;
    isException: boolean;
    isAbsent: boolean;
    notes?: string;
    isHome: boolean;
    now: Date;
  }) => (
    <div
      data-testid="arrival-time-row"
      data-arrival-time={arrivalTime ?? ""}
      data-is-exception={String(isException)}
      data-is-absent={String(isAbsent)}
      data-is-home={String(isHome)}
    >
      {isAbsent && <>Kommt heute nicht</>}
      {!isAbsent && arrivalTime && <>Ankunftszeit: {arrivalTime} Uhr</>}
      {!isAbsent && !arrivalTime && <>Ankunftszeit: —</>}
      {notes && <span>({notes})</span>}
    </div>
  ),
  StudentAbsenceRow: ({ label }: { label: string }) => (
    <div data-testid="student-absence-row">Kommt heute nicht ({label})</div>
  ),
}));

// Mock SWR hook
vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(() => ({
    data: null,
    isLoading: true,
    error: null,
    mutate: vi.fn(),
    isValidating: false,
  })),
  useTenantMutate: vi.fn(() => vi.fn()),
}));

import { useSWRAuth } from "~/lib/swr";
import MeinRaumPage from "./page";

describe("MeinRaumPage (Active Supervisions) (3/5)", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    navigationMockState.roomParam = null;
    global.fetch = vi.fn();
    // Default mock: loading state
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: true,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);
  });

  afterEach(() => {
    cleanup();
  });

  it("shows the spontaneous activity start banner when the capability is enabled", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        supervisedGroups: [],
        unclaimedGroups: [],
        currentStaff: { id: "1" },
        educationalGroups: [],
        firstRoomVisits: [],
        firstRoomId: null,
        capabilities: { webSpontaneousActivitiesEnabled: true },
        plannedNow: [],
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<MeinRaumPage />);

    expect(
      await screen.findByRole("button", {
        name: /Spontane Aktivität starten/,
      }),
    ).toBeInTheDocument();
  });

  it("shows Schulhof in the room picker without opening a dead-end view when status is unavailable", async () => {
    const dashboardResult = {
      data: {
        supervisedGroups: [],
        unclaimedGroups: [],
        currentStaff: { id: "staff-1" },
        educationalGroups: [],
        firstRoomVisits: [],
        firstRoomId: null,
        capabilities: { webSpontaneousActivitiesEnabled: true },
        schulhofStatus: null,
        plannedNow: [],
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    };
    vi.mocked(useSWRAuth).mockReturnValue(dashboardResult as never);
    global.fetch = vi.fn().mockResolvedValue({
      json: async () => ({
        data: [
          { id: 3, name: "Mensa" },
          { id: 5, name: "Schulhof" },
        ],
      }),
    }) as never;

    render(<MeinRaumPage />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: /Spontane Aktivität starten/,
      }),
    );
    fireEvent.click(await screen.findByRole("combobox", { name: "Raum" }));

    expect(
      await screen.findByRole("option", {
        name: "Schulhof (Aufsicht nicht verfügbar)",
      }),
    ).toBeDisabled();
    expect(mockPush).not.toHaveBeenCalledWith(
      "/active-supervisions?room=schulhof",
    );
  });

  it("clears a stale Schulhof shortcut when dashboard revalidation fails", async () => {
    const baseDashboardData = {
      supervisedGroups: [],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: null,
      capabilities: { webSpontaneousActivitiesEnabled: true },
      plannedNow: [],
    };
    let dashboardResult: {
      data: typeof baseDashboardData & { schulhofStatus: unknown };
      isLoading: boolean;
      error: Error | null;
      mutate: typeof mockMutate;
      isValidating: boolean;
    } = {
      data: {
        ...baseDashboardData,
        schulhofStatus: {
          exists: true,
          roomId: "5",
          roomName: "Schulhof",
          activityGroupId: null,
          activeGroupId: null,
          isUserSupervising: false,
          supervisionId: null,
          supervisorCount: 0,
          studentCount: 0,
          supervisors: [],
        },
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    };
    const emptyResult = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    };
    vi.mocked(useSWRAuth).mockImplementation(((key: string | null) =>
      key?.startsWith("active-supervision-dashboard")
        ? dashboardResult
        : emptyResult) as never);
    global.fetch = vi.fn().mockResolvedValue({
      json: async () => ({
        data: [
          { id: 3, name: "Mensa" },
          { id: 5, name: "Schulhof" },
        ],
      }),
    }) as never;

    const { rerender } = render(<MeinRaumPage />);
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /Spontane Aktivität starten/ }),
      ).toBeEnabled();
    });

    dashboardResult = {
      ...dashboardResult,
      error: new Error("dashboard unavailable"),
    };
    rerender(<MeinRaumPage />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: /Spontane Aktivität starten/,
      }),
    );
    fireEvent.click(await screen.findByRole("combobox", { name: "Raum" }));

    expect(
      await screen.findByRole("option", {
        name: "Schulhof (Aufsicht nicht verfügbar)",
      }),
    ).toBeDisabled();
  });

  it("keeps the spontaneous-activity start button clickable in the Schulhof view (regression #1746 deadlock)", async () => {
    // Without any supervised group, the Schulhof tab is auto-selected
    // (allRooms is empty + a Schulhof exists). The start button used to be
    // hard-disabled whenever the Schulhof view was active, so a user with no
    // other active group could never open the modal — a dead end. The button
    // must stay enabled. An occupied Schulhof (activeGroupId set) stays an
    // explicit shortcut inside the modal instead of disabling the trigger.
    //
    // The two return values are hoisted to stable consts so the mock yields the
    // SAME object reference on every call, exactly as real SWR does. Returning a
    // fresh object literal per call (e.g. inside the arrow body) gives the
    // dashboard data a new identity each render, which retriggers the page's
    // data-dependent effects -> infinite render loop -> act() never settles ->
    // unbounded allocation (OOMs the Vitest worker under coverage). Keep these
    // references stable.
    const dashboardResult = {
      data: {
        supervisedGroups: [],
        unclaimedGroups: [],
        currentStaff: { id: "staff-1" },
        educationalGroups: [],
        firstRoomVisits: [],
        firstRoomId: null,
        capabilities: { webSpontaneousActivitiesEnabled: true },
        schulhofStatus: {
          exists: true,
          roomId: "10",
          roomName: "Schulhof",
          activityGroupId: null,
          activeGroupId: "55", // a group is running -> Schulhof is occupied
          isUserSupervising: false,
          supervisionId: null,
          supervisorCount: 1,
          studentCount: 3,
          supervisors: [],
        },
        plannedNow: [],
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    };
    const emptyResult = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    };
    vi.mocked(useSWRAuth).mockImplementation(((key: string | null) =>
      key?.startsWith("active-supervision-dashboard")
        ? dashboardResult
        : emptyResult) as never);

    render(<MeinRaumPage />);

    const startButton = await screen.findByRole("button", {
      name: /Spontane Aktivität starten/,
    });
    expect(startButton).toBeEnabled();
  });

  it("shows loading state when SWR is loading", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: true,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<MeinRaumPage />);

    // Should show loading state while SWR is loading
    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });
});
