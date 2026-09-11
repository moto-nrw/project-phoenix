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

const defaultSupervisionState = vi.hoisted(() => ({
  supervisedRooms: [],
  isLoadingSupervision: false,
  overviewEnabled: false,
  hasGroups: false,
  isLoadingGroups: false,
  groups: [],
  isSupervising: false,
  refresh: vi.fn(),
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

vi.mock("~/lib/supervision-context", () => ({
  useOptionalSupervision: vi.fn(() => defaultSupervisionState),
}));

// Mock next/navigation
const mockPush = vi.fn();
const mockReplace = vi.fn();
const mockRedirect = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
  useSearchParams: () => ({
    get: (key: string) =>
      key === "room"
        ? navigationMockState.roomParam
        : key === "session"
          ? navigationMockState.sessionParam
          : null,
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
  // The action slot is part of the real Alert: the released-room notice and
  // the reopen banner both carry their action in it, so a stub that drops it
  // would hide the only control on those blocks.
  Alert: ({
    message,
    type,
    action,
  }: {
    message: string;
    type: string;
    action?: React.ReactNode;
  }) => (
    <div data-testid={`alert-${type}`}>
      {message}
      {action}
    </div>
  ),
}));

// Mock Modal and ConfirmationModal
vi.mock("~/components/ui/modal", () => ({
  dialogAriaProps: { role: "dialog" as const, "aria-modal": true },
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

vi.mock("~/components/active-supervisions/timetable-roster", () => ({
  TimetableRosterContent: () => <div data-testid="timetable-roster" />,
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
  ActivityIcon: () => <span data-testid="activity-icon" />,
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
import { useSession } from "next-auth/react";
import { useOptionalSupervision } from "~/lib/supervision-context";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import MeinRaumPage from "./page";

const defaultPageHeader = vi
  .mocked(PageHeaderWithSearch)
  .getMockImplementation()!;

beforeEach(() => {
  navigationMockState.sessionParam = null;
  vi.mocked(useSession)
    .mockReset()
    .mockReturnValue({
      data: { user: { token: "test-token" } },
      status: "authenticated",
    } as never);
  vi.mocked(useOptionalSupervision)
    .mockReset()
    .mockReturnValue(defaultSupervisionState);
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
          openRooms: [
            {
              roomId: "schulhof-r1",
              name: "Schulhof",
              isUserSupervising: true,
              activeGroupIds: ["active-schulhof"],
              studentCount: 3,
              students: [],
            },
          ],
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

  it("clicking the release action opens the modal", async () => {
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
          openRooms: [
            {
              roomId: "schulhof-r1",
              name: "Schulhof",
              isUserSupervising: true,
              activeGroupIds: ["active-schulhof"],
              studentCount: 3,
              students: [],
            },
          ],
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

    // Die Aktion steht seit der Kopfkarten-Umstellung einmal im Kopf.
    const releaseButton = await screen.findByRole("button", {
      name: "Aufsicht abgeben",
    });
    fireEvent.click(releaseButton);

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
          openRooms: [
            {
              roomId: "schulhof-r1",
              name: "Schulhof",
              isUserSupervising: false,
              activeGroupIds: ["g-55"],
              studentCount: 0,
              students: [],
            },
          ],
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
          openRooms: [
            {
              roomId: "schulhof-r1",
              name: "Schulhof",
              isUserSupervising: false,
              activeGroupIds: ["g-55"],
              studentCount: 0,
              students: [],
            },
          ],
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
      expect(screen.getByText("Wird übernommen…")).toBeInTheDocument();
    });
  });
});

/**
 * Tab switching between own supervisions and released rooms (#3065).
 */
describe("open-room tab onTabChange callback", () => {
  const mockMutate = vi.fn();
  const originalInnerWidth = window.innerWidth;

  beforeEach(async () => {
    vi.clearAllMocks();
    navigationMockState.roomParam = null;
    navigationMockState.sessionParam = null;
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

  it("clicking a released room's tab opens it by its real room id", async () => {
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
      selectedGroupId: "room-1",
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
      openRooms: [
        {
          roomId: "schulhof-r1",
          name: "Schulhof",
          isUserSupervising: true,
          activeGroupIds: ["active-schulhof"],
          studentCount: 5,
          students: [],
        },
      ],
    };

    // Key-aware mock: the dashboard data must stay available across
    // re-renders so the selection-reconciliation effect (#2096) can compare
    // the cached selectedGroupId against the Schulhof session.
    vi.mocked(useSWRAuth).mockImplementation(((key: unknown) =>
      typeof key === "string" && key.startsWith("active-supervision-dashboard")
        ? ({
            data: dashboardData,
            isLoading: false,
            error: null,
            mutate: mockMutate,
            isValidating: false,
          } as never)
        : ({
            data: null,
            isLoading: false,
            error: null,
            mutate: mockMutate,
            isValidating: false,
          } as never)) as never);

    render(<MeinRaumPage />);

    // Wait for tabs to render
    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "Schulhof" })).toBeInTheDocument();
    });

    // A released room is addressed by its room id, never by a synthetic tab
    // id — several sessions can run in it, so no session identifies it.
    const schulhofTab = screen.getByRole("tab", { name: "Schulhof" });
    fireEvent.click(schulhofTab);

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(
        "/test-tenant/active-supervisions?room=schulhof-r1",
      );
    });

    // The room's occupancy already arrived with the dashboard, so opening it
    // starts no visits request of its own.
    expect(
      activeService.getActiveGroupVisitsWithDisplay,
    ).not.toHaveBeenCalled();
    expect(
      vi
        .mocked(useSWRAuth)
        .mock.calls.some(
          ([key]) =>
            typeof key === "string" && key.startsWith("supervision-visits-"),
        ),
    ).toBe(false);
  });

  it("keeps an own session distinct when its id matches a released room id", async () => {
    const dashboardData = {
      supervisedGroups: [
        {
          id: "1",
          name: "Eigene Aufsicht",
          room_id: "eigener-raum",
          room: { id: "eigener-raum", name: "Klassenraum" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: "1",
      selectedGroupId: "1",
      capabilities: { webSpontaneousActivitiesEnabled: true },
      schulhofStatus: null,
      openRooms: [
        {
          roomId: "1",
          name: "Schulhof",
          isUserSupervising: false,
          activeGroupIds: [],
          studentCount: 0,
          students: [],
        },
      ],
    };

    vi.mocked(useSWRAuth).mockImplementation(((key: unknown) =>
      typeof key === "string" && key.startsWith("active-supervision-dashboard")
        ? ({
            data: dashboardData,
            isLoading: false,
            error: null,
            mutate: mockMutate,
            isValidating: false,
          } as never)
        : ({
            data: null,
            isLoading: false,
            error: null,
            mutate: mockMutate,
            isValidating: false,
          } as never)) as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("tab", { name: /Eigene Aufsicht/ }),
      ).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("tab", { name: /Eigene Aufsicht/ }));

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(
        "/test-tenant/active-supervisions?session=1",
      );
    });
  });

  it("normalizes a session URL in a released room to its shared view", async () => {
    navigationMockState.sessionParam = "active-open";
    const removeItem = vi.spyOn(localStorage, "removeItem");
    const dashboardData = {
      supervisedGroups: [
        {
          id: "active-open",
          name: "Fußball",
          room_id: "sporthalle",
          room: { id: "sporthalle", name: "Sporthalle" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: "active-open",
      selectedGroupId: "active-open",
      capabilities: { webSpontaneousActivitiesEnabled: true },
      schulhofStatus: null,
      openRooms: [
        {
          roomId: "sporthalle",
          name: "Sporthalle",
          isUserSupervising: true,
          activeGroupIds: ["active-open"],
          studentCount: 1,
          students: [
            {
              studentId: "student-1",
              studentName: "Max Muster",
              schoolClass: "2a",
              activeGroupId: "active-open",
              activityName: "Fußball",
              checkInTime: "2026-09-09T10:00:00.000Z",
              isActive: true,
            },
          ],
        },
      ],
    };

    vi.mocked(useSWRAuth).mockImplementation(((key: unknown) =>
      typeof key === "string" && key.startsWith("active-supervision-dashboard")
        ? ({
            data: dashboardData,
            isLoading: false,
            error: null,
            mutate: mockMutate,
            isValidating: false,
          } as never)
        : key === "timetable-roster-active-group-active-open"
          ? ({
              data: {
                instance: { id: "instance-open", activeGroupId: "active-open" },
                rows: [],
              },
              isLoading: false,
              error: null,
              mutate: mockMutate,
              isValidating: false,
            } as never)
          : ({
              data: null,
              isLoading: false,
              error: null,
              mutate: mockMutate,
              isValidating: false,
            } as never)) as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith(
        "/test-tenant/active-supervisions?room=sporthalle",
      );
    });
    expect(screen.getByTestId("timetable-roster")).toBeInTheDocument();
    expect(removeItem).toHaveBeenCalledWith("supervision-last-session");
  });

  it("shows every child in a released room the caller only partly supervises", async () => {
    navigationMockState.roomParam = "sporthalle";
    navigationMockState.sessionParam = "active-fussball";
    const dashboardData = {
      supervisedGroups: [
        {
          id: "active-fussball",
          name: "Fußball",
          room_id: "sporthalle",
          room: { id: "sporthalle", name: "Sporthalle" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: "active-fussball",
      selectedGroupId: "active-fussball",
      capabilities: { webSpontaneousActivitiesEnabled: true },
      schulhofStatus: null,
      openRooms: [
        {
          roomId: "sporthalle",
          name: "Sporthalle",
          isUserSupervising: true,
          activeGroupIds: ["active-fussball", "active-tanzen"],
          studentCount: 2,
          students: [
            {
              studentId: "student-fussball",
              studentName: "Klara Kick",
              schoolClass: "2a",
              activeGroupId: "active-fussball",
              activityName: "Fußball",
              checkInTime: "2026-09-09T10:00:00.000Z",
              isActive: true,
            },
            {
              studentId: "student-tanzen",
              studentName: "Theo Tanz",
              schoolClass: "3b",
              activeGroupId: "active-tanzen",
              activityName: "Tanzen",
              checkInTime: "2026-09-09T10:05:00.000Z",
              isActive: true,
            },
          ],
        },
      ],
    };

    vi.mocked(useSWRAuth).mockImplementation(((key: unknown) =>
      typeof key === "string" && key.startsWith("active-supervision-dashboard")
        ? ({
            data: dashboardData,
            isLoading: false,
            error: null,
            mutate: mockMutate,
            isValidating: false,
          } as never)
        : key === "timetable-roster-active-group-active-fussball"
          ? ({
              data: {
                instance: {
                  id: "instance-fussball",
                  activeGroupId: "active-fussball",
                },
                rows: [],
              },
              isLoading: false,
              error: null,
              mutate: mockMutate,
              isValidating: false,
            } as never)
          : ({
              data: null,
              isLoading: false,
              error: null,
              mutate: mockMutate,
              isValidating: false,
            } as never)) as never);

    render(<MeinRaumPage />);

    expect(await screen.findByText("Klara Kick")).toBeInTheDocument();
    expect(screen.getByText("Theo Tanz")).toBeInTheDocument();
    expect(screen.getByText("Angebot: Fußball")).toBeInTheDocument();
    expect(screen.getByText("Angebot: Tanzen")).toBeInTheDocument();
    expect(screen.queryByTestId("timetable-roster")).not.toBeInTheDocument();
  });

  it("shows a released room the caller does not supervise", async () => {
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
      selectedGroupId: "room-1",
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
      openRooms: [
        {
          roomId: "schulhof-r1",
          name: "Schulhof",
          isUserSupervising: false,
          activeGroupIds: [],
          studentCount: 0,
          students: [],
        },
      ],
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
      expect(screen.getByRole("tab", { name: "Schulhof" })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("tab", { name: "Schulhof" }));

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(
        "/test-tenant/active-supervisions?room=schulhof-r1",
      );
    });

    // The head card names the room an open room and carries no "Eigene
    // Aufsicht" — and the room stays open anyway. Hiding it from everyone but
    // the supervisor is exactly what #3065 removes.
    await waitFor(() => {
      expect(screen.getByText("Offener Raum")).toBeInTheDocument();
    });
    expect(screen.queryByText("Eigene Aufsicht")).not.toBeInTheDocument();

    // Its occupancy came with the dashboard; no extra request is made.
    expect(
      activeService.getActiveGroupVisitsWithDisplay,
    ).not.toHaveBeenCalled();
  });

  it("switching from a released room back to an own session", async () => {
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
      selectedGroupId: "room-1",
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
      openRooms: [
        {
          roomId: "schulhof-r1",
          name: "Schulhof",
          isUserSupervising: true,
          activeGroupIds: ["active-schulhof"],
          studentCount: 5,
          students: [],
        },
      ],
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
      expect(screen.getByRole("tab", { name: "Raum A" })).toBeInTheDocument();
    });

    // Click regular room tab (switching from Schulhof to room)
    fireEvent.click(screen.getByRole("tab", { name: "Raum A" }));

    // Should have called router.push with room URL
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(
        "/test-tenant/active-supervisions?session=room-1",
      );
    });

    // The room's visits ride in the aggregate (#2096) — no separate
    // per-room request is issued by the switch.
    expect(
      activeService.getActiveGroupVisitsWithDisplay,
    ).not.toHaveBeenCalled();
  });
});

describe("RoleGuard integration", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: false,
      error: null,
      mutate: vi.fn(),
      isValidating: false,
    });
  });

  it("shows ForbiddenPage for admin users", async () => {
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockReturnValue({
      data: { user: { token: "test-token", isAdmin: true } },
      status: "authenticated",
    } as never);

    render(<MeinRaumPage />);

    expect(
      screen.getByText("Ihnen fehlt eine Berechtigung"),
    ).toBeInTheDocument();
  });

  it("renders content for non-admin users", async () => {
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockReturnValue({
      data: { user: { token: "test-token", isAdmin: false } },
      status: "authenticated",
    } as never);

    render(<MeinRaumPage />);

    expect(
      screen.queryByText("Ihnen fehlt eine Berechtigung"),
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
  });

  it("renders content for admin with supervised rooms", async () => {
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockReturnValue({
      data: { user: { token: "test-token", isAdmin: true } },
      status: "authenticated",
    } as never);

    // Mock useOptionalSupervision to return overviewEnabled = true,
    // which is the explicit signal that the admin_supervision_overview
    // setting is enabled on the backend.
    vi.mocked(useOptionalSupervision).mockReturnValue({
      supervisedRooms: [{ id: "10", name: "Admin Room", groupId: "1" }],
      isLoadingSupervision: false,
      overviewEnabled: true,
      hasGroups: false,
      isLoadingGroups: false,
      groups: [],
      isSupervising: true,
      refresh: vi.fn(),
    });

    render(<MeinRaumPage />);

    // Admin with rooms should pass the gate and render content
    expect(
      screen.queryByText("Ihnen fehlt eine Berechtigung"),
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
  });

  it("renders content for a non-caregiver confirmed by the overview", async () => {
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: { token: "test-token", isAdmin: false, isCaregiver: false },
      },
      status: "authenticated",
    } as never);

    vi.mocked(useOptionalSupervision).mockReturnValue({
      supervisedRooms: [{ id: "10", name: "Raum", groupId: "1" }],
      isLoadingSupervision: false,
      overviewEnabled: true,
      hasGroups: false,
      isLoadingGroups: false,
      groups: [],
      isSupervising: true,
      refresh: vi.fn(),
    });

    render(<MeinRaumPage />);

    expect(
      screen.queryByText("Ihnen fehlt eine Berechtigung"),
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
  });

  it("blocks admin when only a released room is present (setting off)", async () => {
    // P1-A regression guard — the gate must consult overviewEnabled, not
    // supervisedRooms.length. A released room is reachable for everyone and
    // grants no supervision, so it must not stand for one (#3065).
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockReturnValue({
      data: { user: { token: "test-token", isAdmin: true } },
      status: "authenticated",
    } as never);

    vi.mocked(useOptionalSupervision).mockReturnValue({
      supervisedRooms: [
        {
          id: "schulhof",
          name: "Schulhof",
          groupId: "1",
          isOpenRoom: true,
        },
      ],
      isLoadingSupervision: false,
      overviewEnabled: false,
      hasGroups: false,
      isLoadingGroups: false,
      groups: [],
      isSupervising: true,
      refresh: vi.fn(),
    });

    render(<MeinRaumPage />);

    expect(
      screen.getByText("Ihnen fehlt eine Berechtigung"),
    ).toBeInTheDocument();
  });

  it("shows loading state while supervision is loading for admin", async () => {
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockReturnValue({
      data: { user: { token: "test-token", isAdmin: true } },
      status: "authenticated",
    } as never);

    vi.mocked(useOptionalSupervision).mockReturnValue({
      supervisedRooms: [],
      isLoadingSupervision: true,
      overviewEnabled: false,
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

describe("Aggregate fetcher error contract", () => {
  const mockMutate = vi.fn();

  beforeEach(async () => {
    vi.clearAllMocks();
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockReturnValue({
      data: { user: { token: "test-token", isAdmin: true } },
      status: "authenticated",
    } as never);

    vi.mocked(useOptionalSupervision).mockReturnValue({
      supervisedRooms: [{ id: "10", name: "Admin Room", groupId: "1" }],
      isLoadingSupervision: false,
      overviewEnabled: true,
      hasGroups: false,
      isLoadingGroups: false,
      groups: [],
      isSupervising: true,
      refresh: vi.fn(),
    });
  });

  afterEach(() => cleanup());

  // Capture the dashboard fetcher without invoking it — invoked manually so
  // rejections stay contained in the test instead of escaping to vitest.
  function captureFetcher(): {
    getFetcher: () => (() => Promise<unknown>) | undefined;
  } {
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
    return { getFetcher: () => capturedFetcher };
  }

  it("throws on aggregate failure for admins too — no silent fallback fan-out (#2096)", async () => {
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url.includes("/api/active-supervision-dashboard")) {
        return Promise.resolve({ ok: false, status: 500 });
      }
      return Promise.reject(new Error(`unexpected: ${url}`));
    });
    global.fetch = fetchMock;

    const { getFetcher } = captureFetcher();
    render(<MeinRaumPage />);
    await waitFor(() => expect(getFetcher()).toBeDefined());

    await expect(getFetcher()!()).rejects.toThrow("BFF request failed: 500");

    // The former admin fallback fan-out endpoints must never be contacted:
    // silently-empty partial payloads are the failure mode #2096 removed.
    const urls = fetchMock.mock.calls.map(([u]) => String(u));
    expect(urls.some((u) => u.includes("/api/active/supervisors/all"))).toBe(
      false,
    );
    expect(urls.some((u) => u.includes("/api/active/schulhof/status"))).toBe(
      false,
    );
  });

  it("throws on aggregate failure for non-admins", async () => {
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

    const { getFetcher } = captureFetcher();
    render(<MeinRaumPage />);
    await waitFor(() => expect(getFetcher()).toBeDefined());

    await expect(getFetcher()!()).rejects.toThrow("BFF request failed: 500");
  });
});

const requestBudgetDashboardData = {
  supervisedGroups: [
    { id: "1", name: "Raum 101", room: { id: "10", name: "Raum 101" } },
  ],
  unclaimedGroups: [],
  currentStaff: { id: "1" },
  educationalGroups: [],
  firstRoomVisits: [],
  firstRoomId: "1",
  selectedGroupId: "1",
  capabilities: { webSpontaneousActivitiesEnabled: true },
  schulhofStatus: null,
  openRooms: [],
};

function capturePageLoadFetchers(
  fetchers: Map<string, () => Promise<unknown>>,
) {
  vi.mocked(useSWRAuth).mockImplementation(((
    key: string | null,
    fetcher: (() => Promise<unknown>) | undefined,
  ) => {
    if (typeof key === "string" && fetcher) fetchers.set(key, fetcher);
    return {
      data:
        typeof key === "string" &&
        key.startsWith("active-supervision-dashboard-")
          ? requestBudgetDashboardData
          : null,
      isLoading: false,
      error: null,
      mutate: vi.fn(),
      isValidating: false,
    } as never;
  }) as never);
}

describe("Page-load request budget", () => {
  it("uses one aggregate request plus one selected-session roster request", async () => {
    const fetchers = new Map<string, () => Promise<unknown>>();
    capturePageLoadFetchers(fetchers);
    global.fetch = vi.fn().mockResolvedValue({ ok: false, status: 500 });

    render(<MeinRaumPage />);
    await waitFor(() => expect(fetchers.size).toBe(2));
    await Promise.allSettled(
      [...fetchers.values()].map((fetcher) => fetcher()),
    );

    expect(global.fetch).toHaveBeenCalledTimes(2);
    expect(
      vi.mocked(global.fetch).mock.calls.map(([url]) => String(url)),
    ).toEqual([
      "/api/active-supervision-dashboard?group_id=1",
      "/api/timetable/operations/active-groups/1/roster",
    ]);
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

  it("renders tracking indicators from the aggregate without a separate fetch", async () => {
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
      selectedGroupId: "1",
      // Tracking indicators ride in the aggregate since #2096.
      trackingIndicators: {
        labels: ["Hausaufgaben"],
        results: { "100": [true] },
      },
      schulhofStatus: null,
      openRooms: [],
    };

    vi.mocked(useSWRAuth).mockImplementation(((key: unknown) => {
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
    }) as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // The former separate tracking subscription must be gone (#2096).
    expect(
      vi
        .mocked(useSWRAuth)
        .mock.calls.some(
          (args) =>
            typeof args[0] === "string" &&
            args[0].startsWith("tracking-supervisions"),
        ),
    ).toBe(false);
  });
});
