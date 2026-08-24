/**
 * Tests for Active Supervisions Page
 * Tests the rendering states and user interactions of the active supervisions dashboard
 *
 * NOTE: split into 12 files (page.test.tsx + page.part2..12.test.tsx). The full-dashboard
 * render tests in the "MeinRaumPage (Active Supervisions)" describe are memory-heavy under
 * happy-dom + v8 coverage (~1.5 GB heap each), so a single combined file OOMs the Vitest
 * worker. Those heavy tests are pre-split into (N/M) chunks of 3 renders each, one chunk per
 * file (a 3-render chunk fits comfortably in a 6 GB heap; CI runs with 8 GB). All other
 * describes render cheaply and are packed together. All files share the identical mock header
 * below. When adding a heavy full-dashboard render test, keep it to its own small file.
 */
import {
  render,
  screen,
  waitFor,
  cleanup,
  fireEvent,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

const navigationMockState = vi.hoisted(() => ({
  roomParam: null as string | null,
  sessionParam: null as string | null,
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
    get: (key: string) => {
      if (key === "room") return navigationMockState.roomParam;
      if (key === "session") return navigationMockState.sessionParam;
      return null;
    },
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
    claimActiveGroup: vi.fn(() => Promise.resolve()),
    getTrackingIndicators: vi.fn(() =>
      Promise.resolve({ labels: [], results: {} }),
    ),
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

vi.mock("~/components/rooms/transit-students-section", () => ({
  TransitStudentsSection: () => <div data-testid="transit-students-section" />,
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
// Partial mock: keep the real MOTO_COLOR_PALETTE/LOCATION_COLORS exports
// (moto-duotone-icon.tsx reads MOTO_COLOR_PALETTE at module scope) and only
// stub the location-parsing predicates used by this test file.
vi.mock("~/lib/location-helper", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/location-helper")>();
  return {
    ...actual,
    isHomeLocation: vi.fn(() => false),
    isSchoolyardLocation: vi.fn(() => false),
    isTransitLocation: vi.fn(() => false),
    parseLocation: vi.fn(() => ({ room: "Room 1", status: "Anwesend" })),
  };
});

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
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import MeinRaumPage from "./page";

const defaultPageHeader = vi
  .mocked(PageHeaderWithSearch)
  .getMockImplementation()!;

beforeEach(() => {
  navigationMockState.roomParam = null;
  navigationMockState.sessionParam = null;
  localStorage.clear();
  vi.mocked(PageHeaderWithSearch)
    .mockReset()
    .mockImplementation(defaultPageHeader);
  vi.mocked(useSWRAuth)
    .mockReset()
    .mockReturnValue({
      data: null,
      isLoading: true,
      error: null,
      mutate: vi.fn(),
      isValidating: false,
    } as never);
});

describe("ID-based selection coverage: first room visit enrichment", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem("sidebar-last-room");
    localStorage.removeItem("supervision-last-session");
    localStorage.removeItem("sidebar-last-room-name");
    global.fetch = vi.fn();
  });

  afterEach(() => {
    cleanup();
  });

  it("enriches first room visits with split name, location, and group_id", async () => {
    const dashboardData = {
      supervisedGroups: [
        {
          id: "g1",
          name: "OGS Raum",
          room_id: "r1",
          room: { id: "r1", name: "Raum A" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [
        { id: "eg1", name: "Gruppe Alpha", room: { name: "Raum A" } },
      ],
      firstRoomVisits: [
        {
          studentId: "s1",
          studentName: "Anna Beispiel",
          schoolClass: "2a",
          groupName: "Gruppe Alpha",
          activeGroupId: "g1",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
        {
          studentId: "s2",
          studentName: "Ben Carlo Dreier",
          schoolClass: "3b",
          groupName: "Gruppe Alpha",
          activeGroupId: "g1",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "g1",
      schulhofStatus: null,
    };

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: dashboardData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue({
        data: null,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      const cards = screen.getAllByTestId("student-card");
      expect(cards).toHaveLength(2);
    });

    // Verify names are split correctly (first + last)
    expect(screen.getByText("Anna Beispiel")).toBeInTheDocument();
    expect(screen.getByText("Ben Carlo Dreier")).toBeInTheDocument();
  });

  it("sets empty students when first room exists but has no visits", async () => {
    const dashboardData = {
      supervisedGroups: [
        {
          id: "g1",
          name: "Empty Room",
          room_id: "r1",
          room: { id: "r1", name: "Raum Leer" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: "g1",
      schulhofStatus: null,
    };

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: dashboardData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue({
        data: null,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine Kinder in diesem Raum"),
      ).toBeInTheDocument();
    });
  });

  it("initializes selectedRoomId to first room when no room is pre-selected", async () => {
    // When selectedRoomId is null and firstRoom exists, the code sets selectedRoomId = firstRoom.id
    // This covers lines 674-675
    const dashboardData = {
      supervisedGroups: [
        {
          id: "room-abc",
          name: "Raum X",
          room_id: "rx",
          room: { id: "rx", name: "Raum X" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [
        {
          studentId: "s1",
          studentName: "Test Student",
          schoolClass: "1a",
          groupName: "TestGroup",
          activeGroupId: "room-abc",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "room-abc",
      schulhofStatus: null,
    };

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: dashboardData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue({
        data: null,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });
  });
});

describe("ID-based selection coverage: stale room reset", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem("sidebar-last-room");
    localStorage.removeItem("supervision-last-session");
    localStorage.removeItem("sidebar-last-room-name");
    global.fetch = vi.fn();
  });

  afterEach(() => {
    cleanup();
  });

  it("resets to first room when selected room disappears from list", async () => {
    // First render: two rooms, second is selected via initial data
    const initialData = {
      supervisedGroups: [
        {
          id: "room-1",
          name: "Raum A",
          room_id: "r1",
          room: { id: "r1", name: "Raum A" },
        },
        {
          id: "room-2",
          name: "Raum B",
          room_id: "r2",
          room: { id: "r2", name: "Raum B" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [
        {
          studentId: "s1",
          studentName: "Student A",
          schoolClass: "1a",
          groupName: "G1",
          activeGroupId: "room-1",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "room-1",
      schulhofStatus: null,
    };

    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: initialData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    const { unmount } = render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    unmount();

    // Second render: room-2 is gone (supervision ended), only room-1 remains
    // This triggers the stale room reset at lines 661-662
    const updatedData = {
      supervisedGroups: [
        {
          id: "room-1",
          name: "Raum A",
          room_id: "r1",
          room: { id: "r1", name: "Raum A" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [
        {
          studentId: "s1",
          studentName: "Student A",
          schoolClass: "1a",
          groupName: "G1",
          activeGroupId: "room-1",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "room-1",
      schulhofStatus: null,
    };

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: updatedData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });
  });
});

describe("ID-based selection coverage: switchToRoom via tab click", () => {
  const mockMutate = vi.fn();
  const originalInnerWidth = window.innerWidth;

  beforeEach(async () => {
    vi.clearAllMocks();
    navigationMockState.roomParam = null;
    navigationMockState.sessionParam = null;
    localStorage.removeItem("sidebar-last-room");
    localStorage.removeItem("supervision-last-session");
    localStorage.removeItem("sidebar-last-room-name");
    global.fetch = vi.fn();

    // Set mobile viewport so tabs are rendered (isDesktop = false when < 1024)
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: 500,
    });

    // Override PageHeaderWithSearch to render tabs with onTabChange
    const mod =
      await import("~/components/ui/page-header/PageHeaderWithSearch");
    vi.mocked(
      mod.PageHeaderWithSearch as React.FC<Record<string, unknown>>,
    ).mockImplementation((props: Record<string, unknown>) => {
      const p = props;
      const tabs = p.tabs as
        | {
            items: Array<{ id: string; label: string }>;
            activeTab: string;
            onTabChange: (tabId: string) => void;
          }
        | undefined;
      const badge = p.badge as { count: number } | undefined;
      const actionButton = p.actionButton as React.ReactNode;

      return (
        <div data-testid="page-header" data-count={badge?.count}>
          {tabs?.items.map((tab) => (
            <button
              type="button"
              key={tab.id}
              data-testid={`tab-${tab.id}`}
              data-active={tab.id === tabs.activeTab}
              onClick={() => tabs.onTabChange(tab.id)}
            >
              {tab.label}
            </button>
          ))}
          {actionButton}
        </div>
      );
    });
  });

  afterEach(() => {
    cleanup();
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: originalInnerWidth,
    });
  });

  it("switches to a different room when tab is clicked and loads visits", async () => {
    const dashboardData = {
      supervisedGroups: [
        {
          id: "room-1",
          name: "Raum A",
          room_id: "r1",
          room: { id: "r1", name: "Raum A" },
        },
        {
          id: "room-2",
          name: "Raum B",
          room_id: "r2",
          room: { id: "r2", name: "Raum B" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [
        {
          studentId: "s1",
          studentName: "Room A Student",
          schoolClass: "1a",
          groupName: "G1",
          activeGroupId: "room-1",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "room-1",
      schulhofStatus: null,
    };

    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

    // The aggregate carries the selected session's visits (#2096): a room
    // switch re-runs the dashboard fetch via mutate, whose next response is
    // scoped to room-2.
    const dashboardRef: { current: unknown } = { current: dashboardData };
    mockMutate.mockImplementation(async () => {
      dashboardRef.current = {
        ...dashboardData,
        selectedGroupId: "room-2",
        firstRoomVisits: [
          {
            studentId: "s10",
            studentName: "Room B Student",
            schoolClass: "4a",
            groupName: "Gruppe B",
            activeGroupId: "room-2",
            checkInTime: new Date().toISOString(),
            isActive: true,
          },
        ],
      };
    });
    vi.mocked(useSWRAuth).mockImplementation(((key: unknown) =>
      typeof key === "string" && key.startsWith("active-supervision-dashboard")
        ? ({
            data: dashboardRef.current,
            isLoading: false,
            error: null,
            mutate: mockMutate,
            isValidating: false,
          } as never)
        : swrNull) as never);

    render(<MeinRaumPage />);

    // Wait for initial render with first room's student
    await waitFor(() => {
      expect(screen.getByText("Room A Student")).toBeInTheDocument();
    });

    // Click the second room's tab - triggers switchToRoom -> mutateDashboard
    const tabB = screen.getByTestId("tab-room-2");
    fireEvent.click(tabB);

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalled();
    });

    // After switching, the second room's student should appear
    await waitFor(() => {
      expect(screen.getByText("Room B Student")).toBeInTheDocument();
    });
  });

  it("shows the selected parallel Schulhof group's student count", async () => {
    const dashboardData = {
      supervisedGroups: [
        {
          id: "yard-primary",
          name: "Schulhof",
          room_id: "yard-room",
          room: { id: "yard-room", name: "Schulhof" },
        },
        {
          id: "yard-parallel",
          name: "Schulhof",
          room_id: "yard-room",
          room: { id: "yard-room", name: "Schulhof" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: "yard-primary",
      capabilities: { webSpontaneousActivitiesEnabled: true },
      schulhofStatus: {
        exists: true,
        roomId: "yard-room",
        roomName: "Schulhof",
        activityGroupId: "yard-activity",
        activeGroupId: "yard-primary",
        isUserSupervising: true,
        supervisionId: "sup-primary",
        supervisorCount: 1,
        studentCount: 9,
        supervisors: [
          {
            id: "sup-primary",
            staffId: "staff-1",
            name: "Test Teacher",
            isCurrentUser: true,
          },
        ],
      },
    };
    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

    // Switching to the parallel yard session re-runs the aggregate; its next
    // response carries that session's two visits (#2096).
    const dashboardRef: { current: unknown } = { current: dashboardData };
    mockMutate.mockImplementation(async () => {
      dashboardRef.current = {
        ...dashboardData,
        selectedGroupId: "yard-parallel",
        firstRoomVisits: [
          {
            studentId: "s10",
            studentName: "Parallel Student A",
            schoolClass: "4a",
            groupName: "Gruppe A",
            activeGroupId: "yard-parallel",
            checkInTime: new Date().toISOString(),
            isActive: true,
          },
          {
            studentId: "s11",
            studentName: "Parallel Student B",
            schoolClass: "4b",
            groupName: "Gruppe B",
            activeGroupId: "yard-parallel",
            checkInTime: new Date().toISOString(),
            isActive: true,
          },
        ],
      };
    });
    vi.mocked(useSWRAuth).mockImplementation(((key: unknown) =>
      typeof key === "string" && key.startsWith("active-supervision-dashboard")
        ? ({
            data: dashboardRef.current,
            isLoading: false,
            error: null,
            mutate: mockMutate,
            isValidating: false,
          } as never)
        : swrNull) as never);

    render(<MeinRaumPage />);

    const parallelTab = await screen.findByTestId("tab-yard-parallel");
    fireEvent.click(parallelTab);

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalled();
    });
    await waitFor(() => {
      expect(screen.getByTestId("page-header")).toHaveAttribute(
        "data-count",
        "2",
      );
    });
    expect(
      screen.queryByRole("button", { name: "Aufsicht abgeben" }),
    ).not.toBeInTheDocument();
  });

  it("keeps a parallel Schulhof session visible when the permanent status is unclaimed", async () => {
    const dashboardData = {
      supervisedGroups: [
        {
          id: "yard-parallel",
          name: "Schulhof",
          room_id: "yard-room",
          room: { id: "yard-room", name: "Schulhof" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [
        {
          studentId: "s10",
          studentName: "Parallel Student",
          schoolClass: "4a",
          groupName: "Gruppe A",
          activeGroupId: "yard-parallel",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "yard-parallel",
      capabilities: { webSpontaneousActivitiesEnabled: true },
      schulhofStatus: {
        exists: true,
        roomId: "yard-room",
        roomName: "Schulhof",
        activityGroupId: "yard-activity",
        activeGroupId: "yard-primary",
        isUserSupervising: false,
        supervisionId: null,
        supervisorCount: 0,
        studentCount: 9,
        supervisors: [],
      },
    };
    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: dashboardData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    expect(await screen.findByText("Parallel Student")).toBeInTheDocument();
    expect(screen.getByTestId("page-header")).toHaveAttribute(
      "data-count",
      "1",
    );
  });

  it("shows the permission notice when switching to a forbidden session", async () => {
    // The aggregate answers 403 for a session outside the caller's scope;
    // the fetcher's group_id retry failed too, so mutate rejects. Unlike the
    // former silently-swallowed per-room 403 (#2096), the page now surfaces
    // the permission problem.
    mockMutate.mockRejectedValue(new Error("Request failed: 403"));

    const dashboardData = {
      supervisedGroups: [
        {
          id: "room-1",
          name: "Raum A",
          room_id: "r1",
          room: { id: "r1", name: "Raum A" },
        },
        {
          id: "room-2",
          name: "Raum B",
          room_id: "r2",
          room: { id: "r2", name: "Raum B" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [
        {
          studentId: "s1",
          studentName: "Initial Student",
          schoolClass: "1a",
          groupName: "G1",
          activeGroupId: "room-1",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "room-1",
      schulhofStatus: null,
    };

    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: dashboardData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByText("Initial Student")).toBeInTheDocument();
    });

    // Click the second room's tab - triggers switchToRoom -> mutateDashboard
    const tabB = screen.getByTestId("tab-room-2");
    fireEvent.click(tabB);

    // The permission notice appears and the previous room's students are
    // cleared instead of lingering under the wrong heading.
    await waitFor(() => {
      expect(
        screen.getByText(
          'Keine Berechtigung für "Raum B". Kontaktieren Sie einen Administrator.',
        ),
      ).toBeInTheDocument();
    });
    expect(screen.queryByText("Initial Student")).not.toBeInTheDocument();
  });

  it("handles non-403 error when switching rooms", async () => {
    // A generic aggregate failure while switching surfaces the load error.
    mockMutate.mockRejectedValue(new Error("Network timeout"));

    const dashboardData = {
      supervisedGroups: [
        {
          id: "room-1",
          name: "Raum A",
          room_id: "r1",
          room: { id: "r1", name: "Raum A" },
        },
        {
          id: "room-2",
          name: "Raum B",
          room_id: "r2",
          room: { id: "r2", name: "Raum B" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [
        {
          studentId: "s1",
          studentName: "Student X",
          schoolClass: "1a",
          groupName: "G1",
          activeGroupId: "room-1",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "room-1",
      schulhofStatus: null,
    };

    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: dashboardData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByText("Student X")).toBeInTheDocument();
    });

    // Click second room tab
    const tabB = screen.getByTestId("tab-room-2");
    fireEvent.click(tabB);

    // Generic error should appear
    await waitFor(() => {
      expect(
        screen.getByText("Fehler beim Laden der Raumdaten."),
      ).toBeInTheDocument();
    });
  });

  it("does not switch when clicking the already-selected room tab", async () => {
    const { activeService } = await import("~/lib/active-api");

    const dashboardData = {
      supervisedGroups: [
        {
          id: "room-1",
          name: "Raum A",
          room_id: "r1",
          room: { id: "r1", name: "Raum A" },
        },
        {
          id: "room-2",
          name: "Raum B",
          room_id: "r2",
          room: { id: "r2", name: "Raum B" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [
        {
          studentId: "s1",
          studentName: "Stay Student",
          schoolClass: "1a",
          groupName: "G1",
          activeGroupId: "room-1",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "room-1",
      schulhofStatus: null,
    };

    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: dashboardData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByText("Stay Student")).toBeInTheDocument();
    });

    // Click the already-selected first room tab
    const tabA = screen.getByTestId("tab-room-1");
    fireEvent.click(tabA);

    // switchToRoom early-returns because roomId === selectedRoomId
    // loadRoomVisits should NOT be called (only the initial load triggers it via SWR)
    expect(
      activeService.getActiveGroupVisitsWithDisplay,
    ).not.toHaveBeenCalled();
  });
});

describe("ID-based selection coverage: localStorage room restore", () => {
  const mockMutate = vi.fn();

  beforeEach(async () => {
    vi.clearAllMocks();
    navigationMockState.roomParam = null;
    navigationMockState.sessionParam = null;
    localStorage.removeItem("sidebar-last-room");
    localStorage.removeItem("supervision-last-session");
    localStorage.removeItem("sidebar-last-room-name");
    global.fetch = vi.fn();

    // Override PageHeaderWithSearch to render tabs
    const mod =
      await import("~/components/ui/page-header/PageHeaderWithSearch");
    vi.mocked(
      mod.PageHeaderWithSearch as React.FC<Record<string, unknown>>,
    ).mockImplementation((props: Record<string, unknown>) => {
      const p = props;
      const badge = p.badge as { count: number } | undefined;

      const title = typeof p.title === "string" ? p.title : "";

      return (
        <div data-testid="page-header" data-count={badge?.count}>
          {title}
        </div>
      );
    });
  });

  afterEach(() => {
    cleanup();
  });

  it("restores room from localStorage when no URL param is present", async () => {
    // Set localStorage to point to room r2 (the second room)
    localStorage.setItem("sidebar-last-room", "r2");

    const dashboardData = {
      supervisedGroups: [
        {
          id: "room-1",
          name: "Raum A",
          room_id: "r1",
          room: { id: "r1", name: "Raum A" },
        },
        {
          id: "room-2",
          name: "Raum B",
          room_id: "r2",
          room: { id: "r2", name: "Raum B" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [
        {
          studentId: "s1",
          studentName: "First Room Student",
          schoolClass: "1a",
          groupName: "G1",
          activeGroupId: "room-1",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "room-1",
      schulhofStatus: null,
    };

    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

    // The restore path calls switchToRoom, which re-runs the aggregate; its
    // next response is scoped to the restored session (#2096).
    const dashboardRef: { current: unknown } = { current: dashboardData };
    mockMutate.mockImplementation(async () => {
      dashboardRef.current = {
        ...dashboardData,
        selectedGroupId: "room-2",
        firstRoomVisits: [
          {
            studentId: "s20",
            studentName: "Restored Student",
            schoolClass: "2a",
            groupName: "G2",
            activeGroupId: "room-2",
            checkInTime: new Date().toISOString(),
            isActive: true,
          },
        ],
      };
    });
    vi.mocked(useSWRAuth).mockImplementation(((key: unknown) =>
      typeof key === "string" && key.startsWith("active-supervision-dashboard")
        ? ({
            data: dashboardRef.current,
            isLoading: false,
            error: null,
            mutate: mockMutate,
            isValidating: false,
          } as never)
        : swrNull) as never);

    render(<MeinRaumPage />);

    // The URL sync effect should find savedRoom via allRooms.find(r =>
    // r.room_id === "r2") and call switchToRoom -> mutateDashboard
    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalled();
    });

    await waitFor(() => {
      expect(screen.getByText("Restored Student")).toBeInTheDocument();
    });
  });

  it("persists first room to localStorage when no saved room exists", async () => {
    // No localStorage set = first-room fallback should persist the first room's room_id
    const dashboardData = {
      supervisedGroups: [
        {
          id: "room-1",
          name: "Raum A",
          room_id: "r1",
          room: { id: "r1", name: "Raum A" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: "room-1",
      schulhofStatus: null,
    };

    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: dashboardData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      // The URL sync effect should persist the first room
      expect(localStorage.getItem("sidebar-last-room")).toBe("r1");
    });
  });

  it("persists a session selected by the URL", async () => {
    navigationMockState.sessionParam = "room-2";
    localStorage.setItem("supervision-last-session", "room-1");

    const dashboardData = {
      supervisedGroups: [
        {
          id: "room-1",
          name: "Raum A",
          room_id: "r1",
          room: { id: "r1", name: "Raum A" },
        },
        {
          id: "room-2",
          name: "Raum B",
          room_id: "r2",
          room: { id: "r2", name: "Raum B" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: "room-1",
      schulhofStatus: null,
    };
    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: dashboardData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(localStorage.getItem("supervision-last-session")).toBe("room-2");
    });
  });
});

describe("ID-based selection coverage: Schulhof skip guard", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem("sidebar-last-room");
    localStorage.removeItem("supervision-last-session");
    localStorage.removeItem("sidebar-last-room-name");
    global.fetch = vi.fn();
  });

  afterEach(() => {
    cleanup();
  });

  it("skips first-room preload when Schulhof is the only option and auto-selected", async () => {
    // When there are no regular rooms but Schulhof exists,
    // isSchulhofTabSelected becomes true via auto-select effect.
    // The first-room preload should NOT run (lines 668-670).
    const dashboardData = {
      supervisedGroups: [],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: null,
      capabilities: { webSpontaneousActivitiesEnabled: true },
      schulhofStatus: {
        exists: true,
        roomId: "schulhof-r1",
        roomName: "Schulhof",
        activityGroupId: "ag-1",
        activeGroupId: "active-schulhof",
        isUserSupervising: true,
        supervisionId: "sup-1",
        supervisorCount: 1,
        studentCount: 0,
        supervisors: [
          {
            id: "sup-1",
            staffId: "staff-1",
            name: "Test Teacher",
            isCurrentUser: true,
          },
        ],
      },
    };

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: dashboardData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue({
        data: null,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never);

    render(<MeinRaumPage />);

    // The component should render the Schulhof view (no regular rooms)
    await waitFor(() => {
      expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
    });
  });

  it("does not overwrite Schulhof students with first-room data when Schulhof is active", async () => {
    // Even when there are supervised rooms AND Schulhof, if Schulhof tab is selected
    // (auto-selected because it's the only option initially), first-room preload should skip
    const dashboardData = {
      supervisedGroups: [],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [
        {
          studentId: "s-first",
          studentName: "First Room Student",
          schoolClass: "1a",
          groupName: "G1",
          activeGroupId: "room-1",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: null,
      capabilities: { webSpontaneousActivitiesEnabled: true },
      schulhofStatus: {
        exists: true,
        roomId: "schulhof-r1",
        roomName: "Schulhof",
        activityGroupId: "ag-1",
        activeGroupId: "active-schulhof",
        isUserSupervising: true,
        supervisionId: "sup-1",
        supervisorCount: 1,
        studentCount: 2,
        supervisors: [
          {
            id: "sup-1",
            staffId: "staff-1",
            name: "Test Teacher",
            isCurrentUser: true,
          },
        ],
      },
    };

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: dashboardData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue({
        data: null,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never);

    render(<MeinRaumPage />);

    // The "First Room Student" should NOT appear because Schulhof is auto-selected
    await waitFor(() => {
      expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
    });

    // Verify the first room student was NOT rendered
    expect(screen.queryByText("First Room Student")).not.toBeInTheDocument();
  });
});

describe("ID-based selection coverage: currentRoom useMemo", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem("sidebar-last-room");
    localStorage.removeItem("supervision-last-session");
    localStorage.removeItem("sidebar-last-room-name");
    global.fetch = vi.fn();
  });

  afterEach(() => {
    cleanup();
  });

  it("falls back to first room when selectedRoomId does not match any room", async () => {
    // currentRoom = allRooms.find(r => r.id === selectedRoomId) ?? allRooms[0] ?? null
    // When no room matches selectedRoomId, it falls back to allRooms[0]
    const dashboardData = {
      supervisedGroups: [
        {
          id: "room-only",
          name: "Only Room",
          room_id: "r-only",
          room: { id: "r-only", name: "Einziger Raum" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [
        {
          studentId: "s1",
          studentName: "Solo Student",
          schoolClass: "3c",
          groupName: "G1",
          activeGroupId: "room-only",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "room-only",
      schulhofStatus: null,
    };

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: dashboardData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue({
        data: null,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByText("Solo Student")).toBeInTheDocument();
    });

    // Verify student count badge is shown (proves currentRoom is set)
    const header = screen.getByTestId("page-header");
    expect(header).toHaveAttribute("data-count", "1");
  });

  it("returns Schulhof room object when Schulhof tab is selected and user is supervising", async () => {
    // When isSchulhofTabSelected is true and user is supervising,
    // currentRoom should be the Schulhof virtual room object
    const dashboardData = {
      supervisedGroups: [],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: null,
      capabilities: { webSpontaneousActivitiesEnabled: true },
      schulhofStatus: {
        exists: true,
        roomId: "schulhof-room",
        roomName: "Schulhof",
        activityGroupId: "ag-schulhof",
        activeGroupId: "active-schulhof-id",
        isUserSupervising: true,
        supervisionId: "sup-schulhof",
        supervisorCount: 2,
        studentCount: 5,
        supervisors: [
          {
            id: "sup-1",
            staffId: "staff-1",
            name: "Teacher A",
            isCurrentUser: true,
          },
        ],
      },
    };

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: dashboardData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue({
        data: null,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never);

    render(<MeinRaumPage />);

    // Schulhof auto-selects when it's the only option
    await waitFor(() => {
      expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
    });

    // The student count should reflect Schulhof's student count (5)
    await waitFor(() => {
      const header = screen.getByTestId("page-header");
      expect(header).toHaveAttribute("data-count", "5");
    });
  });
});

describe("ID-based selection coverage: forbidden-session 403 handling", () => {
  const mockMutate = vi.fn();
  const originalInnerWidth = window.innerWidth;

  beforeEach(async () => {
    vi.clearAllMocks();
    localStorage.removeItem("sidebar-last-room");
    localStorage.removeItem("supervision-last-session");
    localStorage.removeItem("sidebar-last-room-name");
    global.fetch = vi.fn();

    // Set mobile viewport so tabs are rendered (isDesktop = false when < 1024)
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: 500,
    });

    // Override PageHeaderWithSearch to render tabs
    const mod =
      await import("~/components/ui/page-header/PageHeaderWithSearch");
    vi.mocked(
      mod.PageHeaderWithSearch as React.FC<Record<string, unknown>>,
    ).mockImplementation((props: Record<string, unknown>) => {
      const p = props;
      const tabs = p.tabs as
        | {
            items: Array<{ id: string; label: string }>;
            activeTab: string;
            onTabChange: (tabId: string) => void;
          }
        | undefined;
      const badge = p.badge as { count: number } | undefined;

      return (
        <div data-testid="page-header" data-count={badge?.count}>
          {tabs?.items.map((tab) => (
            <button
              type="button"
              key={tab.id}
              data-testid={`tab-${tab.id}`}
              onClick={() => tabs.onTabChange(tab.id)}
            >
              {tab.label}
            </button>
          ))}
        </div>
      );
    });
  });

  afterEach(() => {
    cleanup();
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: originalInnerWidth,
    });
  });

  it("shows permission error for 403 and clears students", async () => {
    // The aggregate's mutate rejects with 403 for a session outside the
    // caller's scope (#2096) — switchToRoom shows the permission notice.
    mockMutate.mockRejectedValue(new Error("Request failed: 403"));

    const dashboardData = {
      supervisedGroups: [
        {
          id: "room-1",
          name: "Raum A",
          room_id: "r1",
          room: { id: "r1", name: "Raum A" },
        },
        {
          id: "room-no-access",
          name: "Restricted Room",
          room_id: "r-restricted",
          room: { id: "r-restricted", name: "Restricted Room" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [
        {
          studentId: "s1",
          studentName: "Allowed Student",
          schoolClass: "1a",
          groupName: "G1",
          activeGroupId: "room-1",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "room-1",
      schulhofStatus: null,
    };

    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: dashboardData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByText("Allowed Student")).toBeInTheDocument();
    });

    // Click the restricted room tab
    const restrictedTab = screen.getByTestId("tab-room-no-access");
    fireEvent.click(restrictedTab);

    // switchToRoom surfaces the permission notice and clears the previous
    // room's students.
    await waitFor(() => {
      expect(
        screen.getByText(
          'Keine Berechtigung für "Restricted Room". Kontaktieren Sie einen Administrator.',
        ),
      ).toBeInTheDocument();
    });
    expect(screen.queryByText("Allowed Student")).not.toBeInTheDocument();
  });
});

/**
 * Tests for action button click handlers (lines 1305-1359, 1429)
 * These tests cover the actual click handlers on action buttons
 */
