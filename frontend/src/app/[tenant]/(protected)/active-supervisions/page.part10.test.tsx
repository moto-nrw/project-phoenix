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
import MeinRaumPage from "./page";

describe("Action button click handlers", () => {
  const mockMutate = vi.fn();

  beforeEach(async () => {
    vi.clearAllMocks();
    global.fetch = vi.fn();

    // Override PageHeaderWithSearch to render action buttons as clickable elements
    const mod =
      await import("~/components/ui/page-header/PageHeaderWithSearch");
    vi.mocked(
      mod.PageHeaderWithSearch as React.FC<Record<string, unknown>>,
    ).mockImplementation((props: Record<string, unknown>) => {
      const p = props;
      const actionButton = p.actionButton as React.ReactNode;
      const mobileActionButton = p.mobileActionButton as React.ReactNode;

      return (
        <div data-testid="page-header">
          {actionButton && (
            <div data-testid="action-btn-wrap">{actionButton}</div>
          )}
          {mobileActionButton && (
            <div data-testid="mobile-btn-wrap">{mobileActionButton}</div>
          )}
        </div>
      );
    });
  });

  afterEach(() => {
    cleanup();
  });

  it("clicking release supervision button opens the modal", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
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
            roomId: "schulhof-r1",
            roomName: "Schulhof",
            activityGroupId: "ag-1",
            activeGroupId: "active-schulhof",
            isUserSupervising: true,
            supervisionId: "sup-1",
            supervisorCount: 1,
            studentCount: 3,
            supervisors: [
              {
                id: "sup-1",
                staffId: "staff-1",
                name: "Test Teacher",
                isCurrentUser: true,
              },
            ],
          },
        },
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

    // Wait for release button to appear
    await waitFor(() => {
      expect(screen.getByText("Aufsicht abgeben")).toBeInTheDocument();
    });

    // Click the desktop release button - this triggers setShowReleaseModal(true)
    const releaseButton = screen.getByText("Aufsicht abgeben");
    fireEvent.click(releaseButton);

    // Modal should now be open
    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });
  });

  it("clicking mobile release supervision button opens the modal", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
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
            roomId: "schulhof-r1",
            roomName: "Schulhof",
            activityGroupId: "ag-1",
            activeGroupId: "active-schulhof",
            isUserSupervising: true,
            supervisionId: "sup-1",
            supervisorCount: 1,
            studentCount: 3,
            supervisors: [
              {
                id: "sup-1",
                staffId: "staff-1",
                name: "Test Teacher",
                isCurrentUser: true,
              },
            ],
          },
        },
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

    // Wait for release buttons to appear
    await waitFor(() => {
      expect(
        screen.getAllByLabelText("Aufsicht abgeben").length,
      ).toBeGreaterThanOrEqual(2);
    });

    // Click the mobile release button (second one with aria-label)
    const releaseButtons = screen.getAllByLabelText("Aufsicht abgeben");
    fireEvent.click(releaseButtons[1]!);

    // Modal should now be open
    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });
  });

  it("clicking Beaufsichtigen claims the open Schulhof session", async () => {
    const { activeService } = await import("~/lib/active-api");
    vi.mocked(activeService.claimActiveGroup).mockResolvedValue(
      undefined as never,
    );

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
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
            roomId: "schulhof-r1",
            roomName: "Schulhof",
            activityGroupId: "ag-1",
            activeGroupId: "g-55",
            isUserSupervising: false,
            supervisionId: null,
            supervisorCount: 1,
            studentCount: 0,
            supervisors: [],
          },
        },
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

    // Wait for "Beaufsichtigen" button to appear
    await waitFor(() => {
      expect(screen.getByText("Beaufsichtigen")).toBeInTheDocument();
    });

    // Click the button - this triggers handleToggleSchulhof()
    const beaufsichtigenButton = screen.getByText("Beaufsichtigen");
    fireEvent.click(beaufsichtigenButton);

    // Joins the running session as an additional supervisor (#2161)
    await waitFor(() => {
      expect(activeService.claimActiveGroup).toHaveBeenCalledWith("g-55");
    });
  });

  it("clicking Beaufsichtigen shows loading state", async () => {
    const { activeService } = await import("~/lib/active-api");
    // Make the claim take time
    vi.mocked(activeService.claimActiveGroup).mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 100)),
    );

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
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
            roomId: "schulhof-r1",
            roomName: "Schulhof",
            activityGroupId: "ag-1",
            activeGroupId: "g-55",
            isUserSupervising: false,
            supervisionId: null,
            supervisorCount: 1,
            studentCount: 0,
            supervisors: [],
          },
        },
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
      expect(screen.getByText("Beaufsichtigen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Beaufsichtigen"));

    // Should show loading text
    await waitFor(() => {
      expect(screen.getByText("Wird übernommen...")).toBeInTheDocument();
    });
  });
});

/**
 * Tests for Schulhof tab onTabChange callback (lines 1232-1259)
 */
describe("Schulhof tab onTabChange callback", () => {
  const mockMutate = vi.fn();
  const originalInnerWidth = window.innerWidth;

  beforeEach(async () => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    // Simulate mobile viewport for tabs to appear
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
      const actionButton = p.actionButton as React.ReactNode;

      return (
        <div data-testid="page-header">
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
          {actionButton && (
            <div data-testid="action-btn-wrap">{actionButton}</div>
          )}
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

  it("clicking Schulhof tab triggers onTabChange callback and sets state", async () => {
    const { activeService } = await import("~/lib/active-api");
    vi.mocked(activeService.getActiveGroupVisitsWithDisplay).mockResolvedValue(
      [] as never,
    );

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
        studentCount: 5,
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

    // Wait for tabs to render
    await waitFor(() => {
      expect(screen.getByTestId("tab-schulhof")).toBeInTheDocument();
    });

    // Click the Schulhof tab - triggers onTabChange with "schulhof" (lines 1232-1259)
    const schulhofTab = screen.getByTestId("tab-schulhof");
    fireEvent.click(schulhofTab);

    // Should have called router.push with schulhof URL
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(
        "/test-tenant/active-supervisions?session=schulhof",
      );
    });

    // The tab callback itself must not start a duplicate request.
    expect(
      activeService.getActiveGroupVisitsWithDisplay,
    ).not.toHaveBeenCalled();

    // The keyed SWR subscription owns exactly one Schulhof request and must
    // not carry another room's previous data across the key change.
    const visitCall = vi
      .mocked(useSWRAuth)
      .mock.calls.find(([key]) => key === "supervision-visits-active-schulhof");
    expect(visitCall).toBeDefined();
    expect(visitCall?.[2]).toMatchObject({ keepPreviousData: false });
    const visitFetcher = visitCall?.[1] as (() => Promise<unknown>) | undefined;
    expect(visitFetcher).toBeTypeOf("function");
    await visitFetcher?.();
    expect(activeService.getActiveGroupVisitsWithDisplay).toHaveBeenCalledTimes(
      1,
    );
    expect(activeService.getActiveGroupVisitsWithDisplay).toHaveBeenCalledWith(
      "active-schulhof",
    );
  });

  it("clicking Schulhof tab when not supervising sets empty students", async () => {
    const { activeService } = await import("~/lib/active-api");

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
      capabilities: { webSpontaneousActivitiesEnabled: true },
      schulhofStatus: {
        exists: true,
        roomId: "schulhof-r1",
        roomName: "Schulhof",
        activityGroupId: "ag-1",
        activeGroupId: null, // Not supervising
        isUserSupervising: false,
        supervisionId: null,
        supervisorCount: 0,
        studentCount: 0,
        supervisors: [],
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

    await waitFor(() => {
      expect(screen.getByTestId("tab-schulhof")).toBeInTheDocument();
    });

    // Click Schulhof tab
    fireEvent.click(screen.getByTestId("tab-schulhof"));

    // Should push to schulhof URL
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(
        "/test-tenant/active-supervisions?session=schulhof",
      );
    });

    // Should NOT call getActiveGroupVisitsWithDisplay since not supervising
    expect(
      activeService.getActiveGroupVisitsWithDisplay,
    ).not.toHaveBeenCalled();
  });

  it("switching from Schulhof tab to regular room tab", async () => {
    const { activeService } = await import("~/lib/active-api");
    vi.mocked(activeService.getActiveGroupVisitsWithDisplay).mockResolvedValue(
      [] as never,
    );

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
        studentCount: 5,
        supervisors: [],
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

    await waitFor(() => {
      expect(screen.getByTestId("tab-room-1")).toBeInTheDocument();
    });

    // Click regular room tab (switching from Schulhof to room)
    fireEvent.click(screen.getByTestId("tab-room-1"));

    // Should have called router.push with room URL
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(
        "/test-tenant/active-supervisions?session=room-1",
      );
    });

    // Should have called loadRoomVisits for the room
    await waitFor(() => {
      expect(
        activeService.getActiveGroupVisitsWithDisplay,
      ).toHaveBeenCalledWith("room-1");
    });
  });
});

describe("RoleGuard integration", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows ForbiddenPage for admin users", async () => {
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockReturnValue({
      data: { user: { token: "test-token", isAdmin: true } },
      status: "authenticated",
    } as never);

    render(<MeinRaumPage />);

    expect(screen.getByText("Kein Zugriff")).toBeInTheDocument();
  });

  it("renders content for non-admin users", async () => {
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockReturnValue({
      data: { user: { token: "test-token", isAdmin: false } },
      status: "authenticated",
    } as never);

    render(<MeinRaumPage />);

    expect(screen.queryByText("Kein Zugriff")).not.toBeInTheDocument();
    expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
  });

  it("renders content for admin with supervised rooms", async () => {
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockReturnValue({
      data: { user: { token: "test-token", isAdmin: true } },
      status: "authenticated",
    } as never);

    // Mock useOptionalSupervision to return adminOverviewEnabled = true,
    // which is the explicit signal that the admin_supervision_overview
    // setting is enabled on the backend.
    const supervisionCtx = await import("~/lib/supervision-context");
    vi.spyOn(supervisionCtx, "useOptionalSupervision").mockReturnValue({
      supervisedRooms: [{ id: "10", name: "Admin Room", groupId: "1" }],
      isLoadingSupervision: false,
      adminOverviewEnabled: true,
      hasGroups: false,
      isLoadingGroups: false,
      groups: [],
      isSupervising: true,
      refresh: vi.fn(),
    });

    render(<MeinRaumPage />);

    // Admin with rooms should pass the gate and render content
    expect(screen.queryByText("Kein Zugriff")).not.toBeInTheDocument();
    expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
  });

  it("blocks admin when only a synthetic Schulhof room exists (setting off)", async () => {
    // P1-A regression guard — the gate must consult adminOverviewEnabled,
    // not supervisedRooms.length. A synthetic Schulhof entry is always
    // present when the tenant has a Schulhof, regardless of the setting.
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockReturnValue({
      data: { user: { token: "test-token", isAdmin: true } },
      status: "authenticated",
    } as never);

    const supervisionCtx = await import("~/lib/supervision-context");
    vi.spyOn(supervisionCtx, "useOptionalSupervision").mockReturnValue({
      supervisedRooms: [
        {
          id: "schulhof",
          name: "Schulhof",
          groupId: "1",
          isSchulhof: true,
        },
      ],
      isLoadingSupervision: false,
      adminOverviewEnabled: false,
      hasGroups: false,
      isLoadingGroups: false,
      groups: [],
      isSupervising: true,
      refresh: vi.fn(),
    });

    render(<MeinRaumPage />);

    expect(screen.getByText("Kein Zugriff")).toBeInTheDocument();
  });

  it("shows loading state while supervision is loading for admin", async () => {
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockReturnValue({
      data: { user: { token: "test-token", isAdmin: true } },
      status: "authenticated",
    } as never);

    const supervisionCtx = await import("~/lib/supervision-context");
    vi.spyOn(supervisionCtx, "useOptionalSupervision").mockReturnValue({
      supervisedRooms: [],
      isLoadingSupervision: true,
      adminOverviewEnabled: false,
      hasGroups: false,
      isLoadingGroups: true,
      groups: [],
      isSupervising: false,
      refresh: vi.fn(),
    });

    render(<MeinRaumPage />);

    expect(
      screen.getByLabelText("Aktuelle Aufsicht wird geladen…"),
    ).toBeInTheDocument();
  });
});

describe("Admin fallback dashboard fetcher", () => {
  const mockMutate = vi.fn();

  beforeEach(async () => {
    vi.clearAllMocks();
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockReturnValue({
      data: { user: { token: "test-token", isAdmin: true } },
      status: "authenticated",
    } as never);

    const supervisionCtx = await import("~/lib/supervision-context");
    vi.spyOn(supervisionCtx, "useOptionalSupervision").mockReturnValue({
      supervisedRooms: [{ id: "10", name: "Admin Room", groupId: "1" }],
      isLoadingSupervision: false,
      adminOverviewEnabled: true,
      hasGroups: false,
      isLoadingGroups: false,
      groups: [],
      isSupervising: true,
      refresh: vi.fn(),
    });
  });

  afterEach(() => cleanup());

  // Helper: mock useSWRAuth so the dashboard fetcher actually runs
  function captureFetcher(): {
    getPromise: () => Promise<unknown> | undefined;
  } {
    let dashboardPromise: Promise<unknown> | undefined;
    vi.mocked(useSWRAuth).mockImplementation(((
      key: string | null,
      fetcher: (() => Promise<unknown>) | undefined,
    ) => {
      if (
        key?.startsWith("active-supervision-dashboard") &&
        fetcher &&
        !dashboardPromise
      ) {
        dashboardPromise = fetcher();
      }
      return {
        data: null,
        isLoading: true,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never;
    }) as never);
    return { getPromise: () => dashboardPromise };
  }

  it("populates supervisedGroups and schulhofStatus from fallback endpoints", async () => {
    global.fetch = vi.fn().mockImplementation((url: string) => {
      if (url.includes("/api/active-supervision-dashboard")) {
        return Promise.resolve({ ok: false, status: 500 });
      }
      if (url.includes("/api/active/supervisors/all")) {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({
              data: [
                {
                  id: 1,
                  name: "Group 1",
                  room_id: 10,
                  room: { id: 10, name: "Kunstraum" },
                },
              ],
            }),
        });
      }
      if (url.includes("/api/active/schulhof/status")) {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({
              data: {
                data: {
                  exists: true,
                  room_id: 99,
                  room_name: "Schulhof",
                  active_group_id: 42,
                  is_user_supervising: true,
                  supervision_id: 123,
                  supervisor_count: 1,
                  student_count: 5,
                  supervisors: [
                    {
                      id: 1,
                      staff_id: 2,
                      name: "Super",
                      is_current_user: true,
                    },
                  ],
                },
              },
            }),
        });
      }
      return Promise.reject(new Error(`unexpected: ${url}`));
    });

    const { getPromise } = captureFetcher();
    render(<MeinRaumPage />);
    await waitFor(() => expect(getPromise()).toBeDefined());

    const result = (await getPromise()) as {
      supervisedGroups: Array<{ id: string; room_id?: string }>;
      schulhofStatus: { exists: boolean; supervisorCount: number } | null;
      firstRoomId: string | null;
    };
    expect(result.supervisedGroups).toHaveLength(1);
    expect(result.supervisedGroups[0]?.room_id).toBe("10");
    expect(result.firstRoomId).toBe("10");
    expect(result.schulhofStatus?.exists).toBe(true);
    expect(result.schulhofStatus?.supervisorCount).toBe(1);
  });

  it("returns empty supervisedGroups when supervisors endpoint fails", async () => {
    global.fetch = vi.fn().mockImplementation((url: string) => {
      if (url.includes("/api/active-supervision-dashboard")) {
        return Promise.resolve({ ok: false, status: 403 });
      }
      if (url.includes("/api/active/supervisors/all")) {
        return Promise.resolve({ ok: false });
      }
      if (url.includes("/api/active/schulhof/status")) {
        return Promise.reject(new Error("network"));
      }
      return Promise.reject(new Error("unexpected"));
    });

    const { getPromise } = captureFetcher();
    render(<MeinRaumPage />);
    await waitFor(() => expect(getPromise()).toBeDefined());

    const result = (await getPromise()) as {
      supervisedGroups: unknown[];
      schulhofStatus: unknown;
      firstRoomId: string | null;
    };
    expect(result.supervisedGroups).toEqual([]);
    expect(result.schulhofStatus).toBeNull();
    expect(result.firstRoomId).toBeNull();
  });

  it("omits schulhofStatus when schulhof.exists is false", async () => {
    global.fetch = vi.fn().mockImplementation((url: string) => {
      if (url.includes("/api/active-supervision-dashboard")) {
        return Promise.resolve({ ok: false, status: 500 });
      }
      if (url.includes("/api/active/supervisors/all")) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ data: [] }),
        });
      }
      if (url.includes("/api/active/schulhof/status")) {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({
              data: {
                data: {
                  exists: false,
                  room_name: "",
                  is_user_supervising: false,
                  supervisor_count: 0,
                  student_count: 0,
                },
              },
            }),
        });
      }
      return Promise.reject(new Error("unexpected"));
    });

    const { getPromise } = captureFetcher();
    render(<MeinRaumPage />);
    await waitFor(() => expect(getPromise()).toBeDefined());

    const result = (await getPromise()) as { schulhofStatus: unknown };
    expect(result.schulhofStatus).toBeNull();
  });

  it("skips fallback entirely for non-admin and throws on BFF failure", async () => {
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockReturnValue({
      data: { user: { token: "test-token", isAdmin: false } },
      status: "authenticated",
    } as never);

    global.fetch = vi.fn().mockImplementation((url: string) => {
      if (url.includes("/api/active-supervision-dashboard")) {
        return Promise.resolve({ ok: false, status: 500 });
      }
      return Promise.reject(new Error("should not be called"));
    });

    // Capture the fetcher without invoking — invoke manually below to contain
    // the rejection inside the test instead of letting it escape to vitest.
    let capturedFetcher: (() => Promise<unknown>) | undefined;
    vi.mocked(useSWRAuth).mockImplementation(((
      key: string | null,
      fetcher: (() => Promise<unknown>) | undefined,
    ) => {
      if (
        key?.startsWith("active-supervision-dashboard") &&
        fetcher &&
        !capturedFetcher
      ) {
        capturedFetcher = fetcher;
      }
      return {
        data: null,
        isLoading: true,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never;
    }) as never);

    render(<MeinRaumPage />);
    await waitFor(() => expect(capturedFetcher).toBeDefined());

    await expect(capturedFetcher!()).rejects.toThrow("BFF request failed: 500");
  });
});

describe("Tracking indicators rendering", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    localStorage.clear();
  });

  afterEach(() => cleanup());

  it("subscribes to the tracking hook once a room has students", async () => {
    const dashboardData = {
      supervisedGroups: [
        { id: "1", name: "Raum 101", room: { id: "10", name: "Raum 101" } },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "1" },
      educationalGroups: [],
      firstRoomVisits: [
        {
          studentId: "100",
          studentName: "Max Mustermann",
          schoolClass: "1a",
          groupName: "OGS",
          activeGroupId: "1",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "1",
      schulhofStatus: null,
    };

    vi.mocked(useSWRAuth).mockImplementation((key) => {
      if (typeof key === "string" && key.startsWith("tracking-supervisions")) {
        return {
          data: { labels: ["Hausaufgaben"], results: { "100": [true] } },
          isLoading: false,
          error: null,
          mutate: mockMutate,
          isValidating: false,
        } as never;
      }
      if (
        typeof key === "string" &&
        key.startsWith("active-supervision-dashboard")
      ) {
        return {
          data: dashboardData,
          isLoading: false,
          error: null,
          mutate: mockMutate,
          isValidating: false,
        } as never;
      }
      return {
        data: null,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never;
    });

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    const trackingCall = vi
      .mocked(useSWRAuth)
      .mock.calls.find(
        (args) =>
          typeof args[0] === "string" &&
          args[0].startsWith("tracking-supervisions"),
      );
    expect(trackingCall).toBeDefined();
    expect(trackingCall?.[0]).toContain("tracking-supervisions-");
    expect(trackingCall?.[0]).toContain("100");
  });
});
