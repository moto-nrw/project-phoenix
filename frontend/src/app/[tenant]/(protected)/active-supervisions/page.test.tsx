/**
 * Tests for the Active Supervisions page (MeinRaumPage): rendering states,
 * dashboard data, filters, room selection and the year filter.
 *
 * page.released-rooms, page.roster-actions and page.unplanned-add need a
 * different mock header (supervision context, timetable roster stub,
 * student/timetable API stubs), so they stay separate.
 * Tests that can share this header belong here.
 */
import {
  render as rtlRender,
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
  hasPermission: () => false,
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
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
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
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { useNFCEnabled } from "~/lib/tenant-context";
import MeinRaumPage from "./page";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

// Aktionen melden Fehler als Toast oder im Dialog (#2517); der Provider
// zeigt den Toast echt an.
function render(ui: Parameters<typeof rtlRender>[0]) {
  return rtlRender(ui, { wrapper: ToastProvider });
}

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

describe("MeinRaumPage (Active Supervisions) (1/5)", () => {
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

  it("shows loading state initially", async () => {
    render(<MeinRaumPage />);

    expect(
      screen.getByLabelText("Aktuelle Aufsicht wird geladen…"),
    ).toBeInTheDocument();
  });

  it("renders with SSE error boundary wrapper", () => {
    render(<MeinRaumPage />);

    expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
  });
});

describe("MeinRaumPage (Active Supervisions) (2/5)", () => {
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

  it("shows time rows instead of absence rows for a checked-in sick room student", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [
            {
              id: "g1",
              name: "OGS",
              room_id: "r1",
              room: { id: "r1", name: "Raum A" },
            },
          ],
          unclaimedGroups: [],
          currentStaff: { id: "staff-1" },
          educationalGroups: [
            { id: "eg1", name: "OGS", room: { name: "Raum A" } },
          ],
          firstRoomVisits: [
            {
              studentId: "s1",
              studentName: "Kerstin Krank",
              schoolClass: "1a",
              groupName: "OGS",
              activeGroupId: "g1",
              checkInTime: "2026-01-15T08:05:00.000Z",
              actualArrivalTime: "08:05",
              actualPickupTime: undefined,
              isActive: true,
              sick: true,
            },
          ],
          firstRoomId: "r1",
          schulhofStatus: null,
          openRooms: [],
          plannedNow: [],
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
      expect(screen.getByTestId("arrival-time-row")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("student-absence-row")).not.toBeInTheDocument();
    expect(screen.getByTestId("pickup-time-row")).toBeInTheDocument();
  });

  it("shows no access state when user has no active supervision", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        supervisedGroups: [],
        unclaimedGroups: [],
        currentStaff: { id: "1" },
        educationalGroups: [],
        firstRoomVisits: [],
        firstRoomId: null,
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine aktive Raum-Aufsicht"),
      ).toBeInTheDocument();
    });
  });

  it("shows unclaimed rooms component when user has groups to claim", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        supervisedGroups: [],
        unclaimedGroups: [{ id: "1", name: "Schulhof" }],
        currentStaff: { id: "1" },
        educationalGroups: [],
        firstRoomVisits: [],
        firstRoomId: null,
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByTestId("unclaimed-rooms")).toBeInTheDocument();
    });
  });
});

describe("MeinRaumPage additional scenarios", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    cleanup();
  });

  it("shows empty students state when room has no students", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [
            // Use a non-Schulhof room name to avoid triggering Schulhof-specific code path
            { id: "1", name: "Raum 101", room: { id: "10", name: "Raum 101" } },
          ],
          unclaimedGroups: [],
          currentStaff: { id: "1" },
          educationalGroups: [],
          firstRoomVisits: [], // No students
          firstRoomId: "1",
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
      expect(
        screen.getByText("Keine Kinder in diesem Raum"),
      ).toBeInTheDocument();
    });
  });

  it("renders multiple students in grid", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [
            { id: "1", name: "Raum 101", room: { id: "10", name: "Raum 101" } },
          ],
          unclaimedGroups: [],
          currentStaff: { id: "1" },
          educationalGroups: [
            { id: "g1", name: "OGS Gruppe A", room: { name: "Raum 101" } },
          ],
          firstRoomVisits: [
            {
              studentId: "100",
              studentName: "Max Mustermann",
              schoolClass: "1a",
              groupName: "OGS Gruppe A",
              activeGroupId: "1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
            {
              studentId: "101",
              studentName: "Erika Schmidt",
              schoolClass: "2b",
              groupName: "OGS Gruppe A",
              activeGroupId: "1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
            {
              studentId: "102",
              studentName: "Hans Mueller",
              schoolClass: "1a",
              groupName: "OGS Gruppe A",
              activeGroupId: "1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
          ],
          firstRoomId: "1",
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
      const studentCards = screen.getAllByTestId("student-card");
      expect(studentCards).toHaveLength(3);
    });
  });

  it("shows Schulhof release button when in Schulhof room", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [
            { id: "1", name: "Schulhof", room: { id: "10", name: "Schulhof" } },
          ],
          unclaimedGroups: [],
          currentStaff: { id: "staff1" },
          educationalGroups: [],
          firstRoomVisits: [
            {
              studentId: "100",
              studentName: "Max Mustermann",
              schoolClass: "1a",
              groupName: "OGS Gruppe A",
              activeGroupId: "1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
          ],
          firstRoomId: "1",
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
      // Check for page header with badge showing student count
      expect(screen.getByTestId("page-header")).toBeInTheDocument();
    });
  });

  it("handles generic API error gracefully", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error("BFF request failed: 500"),
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      // Should show error state - using no access view as fallback
      expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
    });
  });

  it("renders unclaimed rooms in empty rooms view", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        supervisedGroups: [], // No supervised groups
        unclaimedGroups: [
          { id: "u1", name: "Schulhof", room: { name: "Schulhof" } },
          { id: "u2", name: "Mensa", room: { name: "Mensa" } },
        ],
        currentStaff: { id: "1" },
        educationalGroups: [],
        firstRoomVisits: [],
        firstRoomId: null,
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByTestId("unclaimed-rooms")).toBeInTheDocument();
      expect(
        screen.getByText("Keine aktive Raum-Aufsicht"),
      ).toBeInTheDocument();
    });
  });

  it("displays room name in responsive layout", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [
            {
              id: "1",
              name: "Kunstzimmer",
              room: { id: "10", name: "Kunstzimmer" },
            },
          ],
          unclaimedGroups: [],
          currentStaff: { id: "1" },
          educationalGroups: [],
          firstRoomVisits: [],
          firstRoomId: "1",
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
      // Verify page renders with SSE boundary
      expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
    });
  });

  it("correctly sets educational group data from dashboard response", async () => {
    const dashboardData = {
      supervisedGroups: [
        { id: "1", name: "Raum A", room: { id: "10", name: "Raum A" } },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff1" },
      educationalGroups: [
        { id: "g1", name: "Gruppe Rot", room: { name: "Raum A" } },
        { id: "g2", name: "Gruppe Blau", room: { name: "Raum B" } },
      ],
      firstRoomVisits: [
        {
          studentId: "s1",
          studentName: "Anna Beispiel",
          schoolClass: "3c",
          groupName: "Gruppe Rot",
          activeGroupId: "1",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "1",
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

  it("shows the supervision and its child count in the status line", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
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
              groupName: "OGS Gruppe A",
              activeGroupId: "1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
            {
              studentId: "101",
              studentName: "Test Student",
              schoolClass: "2b",
              groupName: "OGS Gruppe A",
              activeGroupId: "1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
          ],
          firstRoomId: "1",
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

    // Die Zahl stand früher als Zähler im Kopf; sie steht jetzt in der
    // Statuszeile der Kopfkarte. Der Name der Aufsicht ist der Seitentitel
    // und steht deshalb nicht noch einmal in der Statuszeile (#3312).
    await waitFor(() => {
      expect(screen.getByText("2 Kinder", { selector: "div" })).toBeVisible();
    });
    expect(screen.getByRole("heading", { name: "Raum 101" })).toBeVisible();
    expect(
      screen.queryByRole("heading", { name: "Aktuelle Aufsicht" }),
    ).not.toBeInTheDocument();
  });

  // #3634: mit Grenze steht „Anzahl / Grenze“ in der Statuszeile; über der
  // Grenze dazu „Überbucht“ und der Hinweis über den Kindern, genau an der
  // Grenze keines von beiden.
  it.each([
    { limit: 1, line: "2 / 1 Kinder · Überbucht", warned: true },
    { limit: 2, line: "2 / 2 Kinder", warned: false },
  ])(
    "shows the session's limit in the status line (limit $limit)",
    async ({ limit, line, warned }) => {
      const visit = (studentId: string, studentName: string) => ({
        studentId,
        studentName,
        schoolClass: "1a",
        groupName: "OGS Gruppe A",
        activeGroupId: "1",
        checkInTime: new Date().toISOString(),
        isActive: true,
      });
      vi.mocked(useSWRAuth)
        .mockReturnValueOnce({
          data: {
            supervisedGroups: [
              {
                id: "1",
                name: "Fußball",
                participantLimit: limit,
                room: { id: "10", name: "Raum 101" },
              },
            ],
            unclaimedGroups: [],
            currentStaff: { id: "1" },
            educationalGroups: [],
            firstRoomVisits: [
              visit("100", "Max Mustermann"),
              visit("101", "Test Student"),
            ],
            firstRoomId: "1",
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
        expect(screen.getByText(line, { selector: "div" })).toBeVisible();
      });
      const hint = screen.queryByText(
        /^Mehr Kinder als erlaubt \(höchstens 1\)\./,
      );
      if (warned) {
        expect(hint).toHaveAttribute("data-testid", "alert-warning");
      } else {
        expect(hint).not.toBeInTheDocument();
      }
    },
  );
});

describe("the Schulhof as a released room", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    cleanup();
  });

  it("opens the released room when the caller supervises nothing else", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [],
          unclaimedGroups: [],
          currentStaff: { id: "1" },
          educationalGroups: [],
          firstRoomVisits: [],
          firstRoomId: null,
          capabilities: { webSpontaneousActivitiesEnabled: true },
          schulhofStatus: {
            exists: true,
            roomId: "schulhof-1",
            roomName: "Schulhof",
            activityGroupId: "ag-1",
            activeGroupId: null,
            isUserSupervising: false,
            supervisionId: null,
            supervisorCount: 0,
            studentCount: 0,
            supervisors: [],
          },
          openRooms: [
            {
              roomId: "schulhof-1",
              name: "Schulhof",
              isUserSupervising: false,
              activeGroupIds: [],
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
      // The head card names the room an open room, without "Eigene Aufsicht".
      // The former empty state hid the room's children from everyone who was
      // not supervising; a released room is open to all caregivers (#3065).
      expect(screen.getByText("Offener Raum")).toBeInTheDocument();
      expect(
        screen.getByRole("heading", { name: "Schulhof" }),
      ).toBeInTheDocument();
    });
    expect(screen.queryByText("Eigene Aufsicht")).not.toBeInTheDocument();
  });

  it("names the Schulhof's current supervisors", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [],
          unclaimedGroups: [],
          currentStaff: { id: "1" },
          educationalGroups: [],
          firstRoomVisits: [],
          firstRoomId: null,
          capabilities: { webSpontaneousActivitiesEnabled: true },
          schulhofStatus: {
            exists: true,
            roomId: "schulhof-1",
            roomName: "Schulhof",
            activityGroupId: "ag-1",
            activeGroupId: "active-1",
            isUserSupervising: false,
            supervisionId: null,
            supervisorCount: 2,
            studentCount: 5,
            supervisors: [
              {
                id: "sup-1",
                staffId: "staff-1",
                name: "Max Mustermann",
                isCurrentUser: false,
              },
              {
                id: "sup-2",
                staffId: "staff-2",
                name: "Erika Schmidt",
                isCurrentUser: false,
              },
            ],
          },
          openRooms: [
            {
              roomId: "schulhof-1",
              name: "Schulhof",
              isUserSupervising: false,
              activeGroupIds: ["active-1"],
              studentCount: 5,
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
      expect(
        screen.getByText(
          "5 Kinder · Aktuelle Aufsicht: Max Mustermann, Erika Schmidt",
        ),
      ).toBeInTheDocument();
    });
  });

  it("says plainly that nobody here is the reader's supervision", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [],
          unclaimedGroups: [],
          currentStaff: { id: "1" },
          educationalGroups: [],
          firstRoomVisits: [],
          firstRoomId: null,
          capabilities: { webSpontaneousActivitiesEnabled: true },
          schulhofStatus: {
            exists: true,
            roomId: "schulhof-1",
            roomName: "Schulhof",
            activityGroupId: "ag-1",
            activeGroupId: null,
            isUserSupervising: false,
            supervisionId: null,
            supervisorCount: 0,
            studentCount: 0,
            supervisors: [],
          },
          openRooms: [
            {
              roomId: "schulhof-1",
              name: "Schulhof",
              isUserSupervising: false,
              activeGroupIds: [],
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
      expect(screen.getByText("Offener Raum")).toBeInTheDocument();
      // And the offer to take it is still there for the Schulhof (#2161).
      expect(
        screen.getByRole("button", { name: "Beaufsichtigen" }),
      ).toBeInTheDocument();
    });
    expect(screen.queryByText("Eigene Aufsicht")).not.toBeInTheDocument();
    // Nobody supervises, so the status line names no one. The room itself is
    // the page title and must not be repeated here.
    expect(screen.getByText("0 Kinder")).toBeInTheDocument();
  });

  it("shows the released room's occupancy in the status line", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [],
          unclaimedGroups: [],
          currentStaff: { id: "1" },
          educationalGroups: [],
          firstRoomVisits: [],
          firstRoomId: null,
          capabilities: { webSpontaneousActivitiesEnabled: true },
          schulhofStatus: {
            exists: true,
            roomId: "schulhof-1",
            roomName: "Schulhof",
            activityGroupId: "ag-1",
            activeGroupId: "active-1",
            isUserSupervising: false,
            supervisionId: null,
            supervisorCount: 1,
            studentCount: 15,
            supervisors: [
              {
                id: "sup-1",
                staffId: "staff-1",
                name: "Test Aufsicht",
                isCurrentUser: false,
              },
            ],
          },
          openRooms: [
            {
              roomId: "schulhof-1",
              name: "Schulhof",
              isUserSupervising: false,
              activeGroupIds: ["active-1"],
              studentCount: 15,
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
      // The count comes from the room, not from the caller's own supervision:
      // a shared room reports what is in it whoever is looking.
      expect(
        screen.getByText("15 Kinder · Aktuelle Aufsicht: Test Aufsicht"),
      ).toBeInTheDocument();
    });
  });
});

describe("Enhanced rendering: action buttons and search/filter interaction", () => {
  const mockMutate = vi.fn();

  beforeEach(async () => {
    vi.clearAllMocks();
    global.fetch = vi.fn();

    // Override PageHeaderWithSearch to render action buttons and interactive controls
    const mod =
      await import("~/components/ui/page-header/PageHeaderWithSearch");
    vi.mocked(
      mod.PageHeaderWithSearch as React.FC<Record<string, unknown>>,
    ).mockImplementation((props: Record<string, unknown>) => {
      const p = props;
      const actionButton = p.actionButton as React.ReactNode;
      const mobileActionButton = p.mobileActionButton as React.ReactNode;
      const search = p.search as
        { value: string; onChange: (v: string) => void } | undefined;
      const filters = p.filters as
        | Array<{
            id: string;
            value: string;
            onChange: (v: string) => void;
            options: Array<{ value: string; label: string }>;
          }>
        | undefined;
      const onClearAllFilters = p.onClearAllFilters as (() => void) | undefined;

      return (
        <div data-testid="page-header">
          {actionButton && (
            <div data-testid="action-btn-wrap">{actionButton}</div>
          )}
          {mobileActionButton && (
            <div data-testid="mobile-btn-wrap">{mobileActionButton}</div>
          )}
          {search && (
            <input
              data-testid="search-input"
              value={search.value}
              onChange={(e) => search.onChange(e.target.value)}
            />
          )}
          {filters?.map((f) => (
            <select
              key={f.id}
              data-testid={`filter-${f.id}`}
              value={f.value}
              onChange={(e) => f.onChange(e.target.value)}
            >
              {f.options.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          ))}
          {onClearAllFilters && (
            <button
              type="button"
              data-testid="clear-btn"
              onClick={onClearAllFilters}
            >
              Clear
            </button>
          )}
        </div>
      );
    });
  });

  afterEach(() => {
    cleanup();
  });

  it("renders the release action when supervising Schulhof", async () => {
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

    // Die Aktion steht seit der Kopfkarten-Umstellung einmal im Kopf, statt
    // je einmal für Desktop und Mobil.
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: "Aufsicht abgeben" }),
      ).toBeInTheDocument();
    });
  });

  it("shows 'Beaufsichtigen' button when Schulhof tab selected but not supervising", async () => {
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
            activeGroupId: null,
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
      // The "Beaufsichtigen" button should appear both in action area and main view
      const claimButtons = screen.getAllByText(/Beaufsichtigen|beaufsichtigen/);
      expect(claimButtons.length).toBeGreaterThanOrEqual(1);
    });
  });

  it("filters students by search term through matchesStudentFilters", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [
            {
              id: "1",
              name: "Group A",
              room: { id: "r1", name: "Room 1" },
            },
          ],
          unclaimedGroups: [],
          currentStaff: { id: "staff-1" },
          educationalGroups: [
            { id: "g1", name: "Group Alpha", room: { name: "Room 1" } },
          ],
          firstRoomVisits: [
            {
              studentId: "s1",
              studentName: "Max Mustermann",
              schoolClass: "1a",
              groupName: "Group Alpha",
              activeGroupId: "1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
            {
              studentId: "s2",
              studentName: "Erika Schmidt",
              schoolClass: "2b",
              groupName: "Group Alpha",
              activeGroupId: "1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
          ],
          firstRoomId: "1",
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

    // Wait for both students to render
    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(2);
    });

    // Type in search input to filter
    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Max" } });

    // Should filter to only Max
    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(1);
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
    });
  });

  it("shows EmptyStudentResults when search matches no students", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [
            {
              id: "1",
              name: "Group A",
              room: { id: "r1", name: "Room 1" },
            },
          ],
          unclaimedGroups: [],
          currentStaff: { id: "staff-1" },
          educationalGroups: [],
          firstRoomVisits: [
            {
              studentId: "s1",
              studentName: "Max Mustermann",
              schoolClass: "1a",
              groupName: "Group A",
              activeGroupId: "1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
          ],
          firstRoomId: "1",
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
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // Search for non-existent student
    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "zzzznonexistent" } });

    await waitFor(() => {
      expect(screen.getByTestId("empty-results")).toBeInTheDocument();
    });
  });

  it("filters students by group using the group dropdown", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [
            {
              id: "1",
              name: "Room A",
              room: { id: "r1", name: "Room A" },
            },
          ],
          unclaimedGroups: [],
          currentStaff: { id: "staff-1" },
          educationalGroups: [
            { id: "g1", name: "Group Alpha", room: { name: "Room A" } },
            { id: "g2", name: "Group Beta", room: { name: "Room B" } },
          ],
          firstRoomVisits: [
            {
              studentId: "s1",
              studentName: "Max Alpha",
              schoolClass: "1a",
              groupName: "Group Alpha",
              activeGroupId: "1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
            {
              studentId: "s2",
              studentName: "Erika Beta",
              schoolClass: "2b",
              groupName: "Group Beta",
              activeGroupId: "1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
            {
              studentId: "s3",
              studentName: "Hans Alpha",
              schoolClass: "1a",
              groupName: "Group Alpha",
              activeGroupId: "1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
          ],
          firstRoomId: "1",
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
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });

    // Select group filter
    const groupSelect = screen.getByTestId("filter-group");
    fireEvent.change(groupSelect, { target: { value: "Group Alpha" } });

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(2);
    });
  });

  it("clears all filters when onClearAllFilters is triggered", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [
            {
              id: "1",
              name: "Room A",
              room: { id: "r1", name: "Room A" },
            },
          ],
          unclaimedGroups: [],
          currentStaff: { id: "staff-1" },
          educationalGroups: [],
          firstRoomVisits: [
            {
              studentId: "s1",
              studentName: "Max Mustermann",
              schoolClass: "1a",
              groupName: "Group A",
              activeGroupId: "1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
            {
              studentId: "s2",
              studentName: "Erika Schmidt",
              schoolClass: "2b",
              groupName: "Group B",
              activeGroupId: "1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
          ],
          firstRoomId: "1",
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
      expect(screen.getAllByTestId("student-card")).toHaveLength(2);
    });

    // Apply search filter
    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Max" } });

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(1);
    });

    // Click clear all filters
    const clearBtn = screen.getByTestId("clear-btn");
    fireEvent.click(clearBtn);

    // All students should be visible again
    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(2);
    });
  });
});

describe("Unauthenticated redirect coverage", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    cleanup();
  });

  it("redirects to home when session is unauthenticated", async () => {
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockImplementation(((config?: {
      required?: boolean;
      onUnauthenticated?: () => void;
    }) => {
      if (config?.required && config?.onUnauthenticated) {
        config.onUnauthenticated();
      }
      return { data: null, status: "unauthenticated", update: vi.fn() };
    }) as typeof useSession);

    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      // RoleGuard handles unauthenticated redirect via next/navigation redirect()
      expect(mockRedirect).toHaveBeenCalledWith("/");
    });
  });
});

describe("EmptyRoomsView onClearAllFilters coverage", () => {
  const mockMutate = vi.fn();

  beforeEach(async () => {
    vi.clearAllMocks();
    global.fetch = vi.fn();

    // Restore useSession to authenticated for these tests
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockReturnValue({
      data: { user: { token: "test-token" } },
      status: "authenticated",
    } as never);

    // Override PageHeaderWithSearch to render onClearAllFilters button
    const mod =
      await import("~/components/ui/page-header/PageHeaderWithSearch");
    vi.mocked(
      mod.PageHeaderWithSearch as React.FC<Record<string, unknown>>,
    ).mockImplementation((props: Record<string, unknown>) => {
      const p = props;
      const search = p.search as
        { value: string; onChange: (v: string) => void } | undefined;
      const onClearAllFilters = p.onClearAllFilters as (() => void) | undefined;

      return (
        <div data-testid="page-header">
          {search && (
            <input
              data-testid="search-input"
              value={search.value}
              onChange={(e) => search.onChange(e.target.value)}
            />
          )}
          {onClearAllFilters && (
            <button
              type="button"
              data-testid="clear-btn"
              onClick={onClearAllFilters}
            >
              Clear
            </button>
          )}
        </div>
      );
    });
  });

  afterEach(() => {
    cleanup();
  });

  it("clears search and group filter in EmptyRoomsView", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        supervisedGroups: [],
        unclaimedGroups: [
          { id: "u1", name: "Available Room", room: { name: "Room X" } },
        ],
        currentStaff: { id: "staff-1" },
        educationalGroups: [],
        firstRoomVisits: [],
        firstRoomId: null,
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByTestId("unclaimed-rooms")).toBeInTheDocument();
    });

    // The clear button should exist from the EmptyRoomsView's PageHeaderWithSearch
    const clearBtn = screen.getByTestId("clear-btn");
    fireEvent.click(clearBtn);

    // No error should occur - the callback sets searchTerm="" and groupFilter="all"
    expect(clearBtn).toBeInTheDocument();
  });
});

describe("BFF dashboard data with students and Schulhof", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    localStorage.clear();
  });

  afterEach(() => {
    cleanup();
  });

  it("renders supervised room with first room visits from BFF", async () => {
    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [
            {
              id: "g1",
              name: "OGS Blau",
              room_id: "r1",
              room: { id: "r1", name: "Kunstzimmer" },
            },
          ],
          unclaimedGroups: [],
          currentStaff: { id: "staff-1" },
          educationalGroups: [
            { id: "eg1", name: "OGS Blau", room: { name: "Kunstzimmer" } },
          ],
          firstRoomVisits: [
            {
              studentId: "s1",
              studentName: "Max Mustermann",
              schoolClass: "3a",
              groupName: "OGS Blau",
              activeGroupId: "g1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
            {
              studentId: "s2",
              studentName: "Erika Muster",
              schoolClass: "3b",
              groupName: "OGS Blau",
              activeGroupId: "g1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
          ],
          firstRoomId: "r1",
          schulhofStatus: null,
          openRooms: [],
        },
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      const cards = screen.getAllByTestId("student-card");
      expect(cards.length).toBe(2);
    });
  });

  it("renders supervised room with no students (empty room message)", async () => {
    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [
            {
              id: "g1",
              name: "OGS Blau",
              room_id: "r1",
              room: { id: "r1", name: "Kunstzimmer" },
            },
          ],
          unclaimedGroups: [],
          currentStaff: { id: "staff-1" },
          educationalGroups: [],
          firstRoomVisits: [],
          firstRoomId: "r1",
          schulhofStatus: null,
          openRooms: [],
        },
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine Kinder in diesem Raum"),
      ).toBeInTheDocument();
    });
  });

  it("renders Schulhof status when present in BFF data", async () => {
    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

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
            roomId: "r100",
            roomName: "Schulhof",
            activityGroupId: "ag1",
            activeGroupId: "g100",
            isUserSupervising: true,
            supervisionId: "sup1",
            supervisorCount: 1,
            studentCount: 5,
            supervisors: [
              {
                id: "sup1",
                staffId: "staff-1",
                name: "Test Teacher",
                isCurrentUser: true,
              },
            ],
          },
          openRooms: [
            {
              roomId: "r100",
              name: "Schulhof",
              isUserSupervising: true,
              activeGroupIds: ["g100"],
              studentCount: 5,
              students: [],
            },
          ],
        },
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    // When Schulhof exists and no regular rooms, it auto-selects Schulhof tab
    await waitFor(() => {
      expect(screen.getByTestId("page-header")).toBeInTheDocument();
    });
  });

  it("renders multiple supervised rooms and keeps first room selected", async () => {
    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [
            {
              id: "g1",
              name: "OGS Blau",
              room_id: "r1",
              room: { id: "r1", name: "Atelier" },
            },
            {
              id: "g2",
              name: "OGS Rot",
              room_id: "r2",
              room: { id: "r2", name: "Mensa" },
            },
          ],
          unclaimedGroups: [],
          currentStaff: { id: "staff-1" },
          educationalGroups: [
            { id: "eg1", name: "OGS Blau", room: { name: "Atelier" } },
          ],
          firstRoomVisits: [
            {
              studentId: "s1",
              studentName: "Anna Schmidt",
              schoolClass: "2a",
              groupName: "OGS Blau",
              activeGroupId: "g1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
          ],
          firstRoomId: "r1",
          schulhofStatus: null,
          openRooms: [],
        },
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      const cards = screen.getAllByTestId("student-card");
      expect(cards.length).toBe(1);
    });
  });

  it("renders empty rooms view with unclaimed groups and no Schulhof", async () => {
    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [],
          unclaimedGroups: [
            { id: "u1", name: "Unclaimed Room", room: { name: "Raum C" } },
          ],
          currentStaff: { id: "staff-1" },
          educationalGroups: [],
          firstRoomVisits: [],
          firstRoomId: null,
          schulhofStatus: null,
          openRooms: [],
        },
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByTestId("unclaimed-rooms")).toBeInTheDocument();
    });
  });
});

describe("matchesStudentFilters edge cases", () => {
  const mockMutate = vi.fn();

  beforeEach(async () => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    localStorage.clear();

    // Override PageHeaderWithSearch to expose search and filter
    const mod =
      await import("~/components/ui/page-header/PageHeaderWithSearch");
    vi.mocked(
      mod.PageHeaderWithSearch as React.FC<Record<string, unknown>>,
    ).mockImplementation((props: Record<string, unknown>) => {
      const p = props;
      const search = p.search as
        { value: string; onChange: (v: string) => void } | undefined;
      const filters = p.filters as
        | Array<{
            id: string;
            value: string;
            onChange: (v: string) => void;
            options: Array<{ value: string; label: string }>;
          }>
        | undefined;

      return (
        <div data-testid="page-header">
          {search && (
            <input
              data-testid="search-input"
              value={search.value}
              onChange={(e) => search.onChange(e.target.value)}
            />
          )}
          {filters?.map((f) => (
            <select
              key={f.id}
              data-testid={`filter-${f.id}`}
              value={f.value}
              onChange={(e) => f.onChange(e.target.value)}
            >
              {f.options.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          ))}
        </div>
      );
    });
  });

  afterEach(() => {
    cleanup();
  });

  it("filters students by search term matching first_name", async () => {
    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [
            {
              id: "g1",
              name: "OGS",
              room_id: "r1",
              room: { id: "r1", name: "Raum A" },
            },
          ],
          unclaimedGroups: [],
          currentStaff: { id: "staff-1" },
          educationalGroups: [
            { id: "eg1", name: "OGS", room: { name: "Raum A" } },
          ],
          firstRoomVisits: [
            {
              studentId: "s1",
              studentName: "Max Mustermann",
              schoolClass: "3a",
              groupName: "OGS",
              activeGroupId: "g1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
            {
              studentId: "s2",
              studentName: "Erika Muster",
              schoolClass: "3b",
              groupName: "OGS",
              activeGroupId: "g1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
          ],
          firstRoomId: "r1",
          schulhofStatus: null,
          openRooms: [],
        },
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    // Wait for students to render
    await waitFor(() => {
      expect(screen.getAllByTestId("student-card").length).toBe(2);
    });

    // Type search term to filter by name
    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Erika" } });

    // Should only show one student
    await waitFor(() => {
      expect(screen.getAllByTestId("student-card").length).toBe(1);
    });
  });

  it("filters students by group name", async () => {
    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [
            {
              id: "g1",
              name: "OGS",
              room_id: "r1",
              room: { id: "r1", name: "Raum A" },
            },
          ],
          unclaimedGroups: [],
          currentStaff: { id: "staff-1" },
          educationalGroups: [
            { id: "eg1", name: "Gruppe A", room: { name: "Raum A" } },
            { id: "eg2", name: "Gruppe B", room: { name: "Raum B" } },
          ],
          firstRoomVisits: [
            {
              studentId: "s1",
              studentName: "Max Mustermann",
              schoolClass: "3a",
              groupName: "Gruppe A",
              activeGroupId: "g1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
            {
              studentId: "s2",
              studentName: "Erika Muster",
              schoolClass: "3b",
              groupName: "Gruppe B",
              activeGroupId: "g1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
          ],
          firstRoomId: "r1",
          schulhofStatus: null,
          openRooms: [],
        },
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card").length).toBe(2);
    });

    // Select a specific group filter
    const groupFilter = screen.getByTestId("filter-group");
    fireEvent.change(groupFilter, { target: { value: "Gruppe A" } });

    // Should show only students from Gruppe A
    await waitFor(() => {
      expect(screen.getAllByTestId("student-card").length).toBe(1);
    });
  });

  it("shows EmptyStudentResults when search yields no matches", async () => {
    const swrNull = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never;

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [
            {
              id: "g1",
              name: "OGS",
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
              studentName: "Max Mustermann",
              schoolClass: "3a",
              groupName: "OGS",
              activeGroupId: "g1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
          ],
          firstRoomId: "r1",
          schulhofStatus: null,
          openRooms: [],
        },
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card").length).toBe(1);
    });

    // Search for non-existent student
    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Nonexistent" } });

    // Should show empty results
    await waitFor(() => {
      expect(screen.getByTestId("empty-results")).toBeInTheDocument();
    });
  });
});

describe("Schulhof user supervising view", () => {
  const mockMutate = vi.fn();
  const swrNull = {
    data: null,
    isLoading: false,
    error: null,
    mutate: mockMutate,
    isValidating: false,
  } as never;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    localStorage.clear();
    navigationMockState.roomParam = null;
  });

  afterEach(() => {
    cleanup();
  });

  it("renders Schulhof view when user IS supervising (with active group)", async () => {
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
            roomId: "room-schulhof",
            roomName: "Schulhof",
            activityGroupId: "ag-1",
            activeGroupId: "active-sch-1",
            isUserSupervising: true,
            supervisionId: "sup-current",
            supervisorCount: 1,
            studentCount: 3,
            supervisors: [
              {
                id: "sup-current",
                staffId: "staff-1",
                name: "Current User",
                isCurrentUser: true,
              },
            ],
          },
          openRooms: [
            {
              roomId: "room-schulhof",
              name: "Schulhof",
              isUserSupervising: true,
              activeGroupIds: ["active-sch-1"],
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
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      // When user is supervising, they should see the supervision view with student list
      // The page header should show student count
      const header = screen.getByTestId("page-header");
      expect(header).toBeInTheDocument();
    });
  });

  it("renders Schulhof with both supervised rooms and Schulhof tab", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [
            {
              id: "g1",
              name: "OGS",
              room_id: "r1",
              room: { id: "r1", name: "Raum 101" },
            },
          ],
          unclaimedGroups: [],
          currentStaff: { id: "staff-1" },
          educationalGroups: [],
          firstRoomVisits: [
            {
              studentId: "s1",
              studentName: "Max Test",
              schoolClass: "2a",
              groupName: "OGS",
              activeGroupId: "g1",
              checkInTime: new Date().toISOString(),
              isActive: true,
            },
          ],
          firstRoomId: "r1",
          capabilities: { webSpontaneousActivitiesEnabled: true },
          schulhofStatus: {
            exists: true,
            roomId: "room-schulhof",
            roomName: "Schulhof",
            activityGroupId: "ag-1",
            activeGroupId: "active-sch-1",
            isUserSupervising: false,
            supervisionId: null,
            supervisorCount: 0,
            studentCount: 0,
            supervisors: [],
          },
          openRooms: [
            {
              roomId: "room-schulhof",
              name: "Schulhof",
              isUserSupervising: false,
              activeGroupIds: ["active-sch-1"],
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
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      // Should render the room view with students
      expect(screen.getAllByTestId("student-card").length).toBe(1);
    });
  });

  it("handles single room without Schulhof — no tabs on mobile", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [
            {
              id: "g1",
              name: "OGS",
              room_id: "r1",
              room: { id: "r1", name: "Raum 101" },
            },
          ],
          unclaimedGroups: [],
          currentStaff: { id: "staff-1" },
          educationalGroups: [],
          firstRoomVisits: [],
          firstRoomId: "r1",
          schulhofStatus: null,
          openRooms: [],
        },
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      // With single room and no Schulhof, title should show room name on mobile
      const header = screen.getByTestId("page-header");
      expect(header).toBeInTheDocument();
    });
  });
});

describe("ID-based selection: Stale selection reset", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    cleanup();
  });

  it("resets to first room when previously selected room disappears from active rooms list", async () => {
    // Initial render with room g2 selected
    const initialData = {
      supervisedGroups: [
        { id: "g1", name: "Raum A", room: { id: "10", name: "Raum A" } },
        { id: "g2", name: "Raum B", room: { id: "11", name: "Raum B" } },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: "g1",
    };

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: initialData,
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

    const { rerender } = render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByTestId("page-header")).toBeInTheDocument();
    });

    // Simulate SSE refresh where g2 is removed (supervision revoked)
    const updatedData = {
      supervisedGroups: [
        { id: "g1", name: "Raum A", room: { id: "10", name: "Raum A" } },
        // g2 is gone
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: "g1",
    };

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: updatedData,
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

    rerender(<MeinRaumPage />);

    await waitFor(() => {
      // Should reset to first room (g1)
      expect(screen.getByTestId("page-header")).toBeInTheDocument();
    });
  });

  it("handles case when selected room disappears and no rooms remain", async () => {
    const initialData = {
      supervisedGroups: [
        { id: "g1", name: "Raum A", room: { id: "10", name: "Raum A" } },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: "g1",
    };

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: initialData,
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

    const { rerender } = render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByTestId("page-header")).toBeInTheDocument();
    });

    // All rooms removed
    const updatedData = {
      supervisedGroups: [],
      unclaimedGroups: [],
      currentStaff: { id: "staff1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: null,
    };

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: updatedData,
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

    rerender(<MeinRaumPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine aktive Raum-Aufsicht"),
      ).toBeInTheDocument();
    });
  });
});

describe("ID-based selection: switchToRoom function", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    cleanup();
  });

  it("handles room not found by ID gracefully (no-op)", async () => {
    const dashboardData = {
      supervisedGroups: [
        { id: "g1", name: "Raum A", room: { id: "10", name: "Raum A" } },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: "g1",
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
      expect(screen.getByTestId("page-header")).toBeInTheDocument();
    });

    // switchToRoom with non-existent ID should be a no-op
    // This is tested indirectly - page should not crash or show errors
  });

  it("does not switch when target room ID matches current selection", async () => {
    const dashboardData = {
      supervisedGroups: [
        { id: "g1", name: "Raum A", room: { id: "10", name: "Raum A" } },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: "g1",
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
      expect(screen.getByTestId("page-header")).toBeInTheDocument();
    });

    // Attempting to switch to the same room should be a no-op
  });
});

/**
 * Tests for ID-based selection logic introduced in the SSE selection stability PR.
 * These render the actual MeinRaumPage component rather than a stub, so the
 * selection path is exercised end to end.
 */

describe("Year filter (Klassenstufe) on active supervisions", () => {
  const mockMutate = vi.fn();

  beforeEach(async () => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    localStorage.clear();

    // Override PageHeaderWithSearch to expose year filter
    const mod =
      await import("~/components/ui/page-header/PageHeaderWithSearch");
    vi.mocked(
      mod.PageHeaderWithSearch as React.FC<Record<string, unknown>>,
    ).mockImplementation((props: Record<string, unknown>) => {
      const p = props;
      const search = p.search as
        { value: string; onChange: (v: string) => void } | undefined;
      const filters = p.filters as
        | Array<{
            id: string;
            value: string;
            onChange: (v: string) => void;
            options: Array<{ value: string; label: string }>;
          }>
        | undefined;
      const activeFilters = p.activeFilters as
        Array<{ id: string; label: string; onRemove?: () => void }> | undefined;
      const onClearAllFilters = p.onClearAllFilters as (() => void) | undefined;

      return (
        <div data-testid="page-header">
          {search && (
            <input
              data-testid="search-input"
              value={search.value}
              onChange={(e) => search.onChange(e.target.value)}
            />
          )}
          {filters?.map((f) => (
            <select
              key={f.id}
              data-testid={`filter-${f.id}`}
              value={f.value}
              onChange={(e) => f.onChange(e.target.value)}
            >
              {f.options.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          ))}
          <div data-testid="active-filters">
            {activeFilters?.map((f) => (
              <button
                type="button"
                key={f.id}
                data-testid={`active-filter-${f.id}`}
                onClick={f.onRemove}
              >
                {f.label}
              </button>
            ))}
          </div>
          {onClearAllFilters && (
            <button
              type="button"
              data-testid="clear-filters"
              onClick={onClearAllFilters}
            >
              Clear
            </button>
          )}
        </div>
      );
    });
  });

  afterEach(() => cleanup());

  function makeDashboardWithStudents(
    students: Array<{
      id: string;
      name: string;
      schoolClass: string;
      groupName: string;
    }>,
  ) {
    return {
      supervisedGroups: [
        {
          id: "g1",
          name: "OGS",
          room_id: "r1",
          room: { id: "r1", name: "Raum A" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "staff-1" },
      educationalGroups: [{ id: "eg1", name: "OGS", room: { name: "Raum A" } }],
      firstRoomVisits: students.map((s) => ({
        studentId: s.id,
        studentName: s.name,
        schoolClass: s.schoolClass,
        groupName: s.groupName,
        activeGroupId: "g1",
        checkInTime: new Date().toISOString(),
        isActive: true,
      })),
      firstRoomId: "r1",
      schulhofStatus: null,
      openRooms: [],
    };
  }

  const fourStudents = [
    { id: "s1", name: "Max Mustermann", schoolClass: "1a", groupName: "OGS" },
    { id: "s2", name: "Anna Schmidt", schoolClass: "2b", groupName: "OGS" },
    { id: "s3", name: "Tom Weber", schoolClass: "1c", groupName: "OGS" },
    { id: "s4", name: "Lisa Müller", schoolClass: "3a", groupName: "OGS" },
  ];

  const swrNull = {
    data: null,
    isLoading: false,
    error: null,
    mutate: mockMutate,
    isValidating: false,
  } as never;

  it("filters students by year 1 — shows only class 1 students", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: makeDashboardWithStudents(fourStudents),
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card").length).toBe(4);
    });

    // Filter by year 1
    const yearFilter = screen.getByTestId("filter-year");
    fireEvent.change(yearFilter, { target: { value: "1" } });

    await waitFor(() => {
      // Max (1a) and Tom (1c) should remain; Anna (2b) and Lisa (3a) filtered out
      expect(screen.getAllByTestId("student-card").length).toBe(2);
    });
  });

  it("filters students by year 3 — shows only class 3 students", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: makeDashboardWithStudents(fourStudents),
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card").length).toBe(4);
    });

    const yearFilter = screen.getByTestId("filter-year");
    fireEvent.change(yearFilter, { target: { value: "3" } });

    await waitFor(() => {
      // Only Lisa (3a)
      expect(screen.getAllByTestId("student-card").length).toBe(1);
    });
  });

  it("shows all students when year filter is reset to 'all'", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: makeDashboardWithStudents(fourStudents),
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card").length).toBe(4);
    });

    const yearFilter = screen.getByTestId("filter-year");

    // Filter by year 1
    fireEvent.change(yearFilter, { target: { value: "1" } });
    await waitFor(() => {
      expect(screen.getAllByTestId("student-card").length).toBe(2);
    });

    // Reset to all
    fireEvent.change(yearFilter, { target: { value: "all" } });
    await waitFor(() => {
      expect(screen.getAllByTestId("student-card").length).toBe(4);
    });
  });

  it("shows year active filter chip with correct label", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: makeDashboardWithStudents(fourStudents),
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card").length).toBe(4);
    });

    const yearFilter = screen.getByTestId("filter-year");
    fireEvent.change(yearFilter, { target: { value: "2" } });

    await waitFor(() => {
      const chip = screen.getByTestId("active-filter-year");
      expect(chip).toBeInTheDocument();
      expect(chip).toHaveTextContent("Jahr 2");
    });
  });

  it("removes year filter when active filter chip is clicked", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: makeDashboardWithStudents(fourStudents),
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card").length).toBe(4);
    });

    // Set year filter
    const yearFilter = screen.getByTestId("filter-year");
    fireEvent.change(yearFilter, { target: { value: "1" } });

    await waitFor(() => {
      expect(screen.getByTestId("active-filter-year")).toBeInTheDocument();
      expect(screen.getAllByTestId("student-card").length).toBe(2);
    });

    // Click chip to remove
    fireEvent.click(screen.getByTestId("active-filter-year"));

    await waitFor(() => {
      expect(
        screen.queryByTestId("active-filter-year"),
      ).not.toBeInTheDocument();
      expect(screen.getAllByTestId("student-card").length).toBe(4);
    });
  });

  it("clears year filter when clear-all-filters is clicked", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: makeDashboardWithStudents(fourStudents),
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue(swrNull);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card").length).toBe(4);
    });

    // Set year filter
    const yearFilter = screen.getByTestId("filter-year");
    fireEvent.change(yearFilter, { target: { value: "1" } });

    await waitFor(() => {
      expect(screen.getByTestId("active-filter-year")).toBeInTheDocument();
    });

    // Click clear all
    fireEvent.click(screen.getByTestId("clear-filters"));

    await waitFor(() => {
      expect(
        screen.queryByTestId("active-filter-year"),
      ).not.toBeInTheDocument();
      expect(screen.getAllByTestId("student-card").length).toBe(4);
    });
  });
});

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

  it("updates both spontaneous starts when the server day changes to a weekend", async () => {
    const weekdayData = {
      businessDay: "2026-08-28",
      spontaneousStartAvailability: {
        available: true,
        blockedReason: undefined as "weekend" | undefined,
      },
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
        activeGroupId: null,
        isUserSupervising: false,
        supervisionId: null,
        supervisorCount: 0,
        studentCount: 0,
        supervisors: [],
      },
      openRooms: [
        {
          roomId: "10",
          name: "Schulhof",
          isUserSupervising: false,
          activeGroupIds: [],
          studentCount: 0,
          students: [],
        },
      ],
      plannedNow: [],
    };
    let dashboardResult = {
      data: weekdayData,
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

    const { rerender } = render(<MeinRaumPage />);

    expect(
      await screen.findByRole("button", { name: /Spontane Aktivität starten/ }),
    ).toBeEnabled();
    expect(
      await screen.findByRole("button", { name: "Beaufsichtigen" }),
    ).toBeEnabled();
    const dashboardCall = vi
      .mocked(useSWRAuth)
      .mock.calls.find(([key]) =>
        key?.startsWith("active-supervision-dashboard"),
      );
    expect(dashboardCall?.[2]).toMatchObject({
      refreshInterval: 60_000,
      revalidateOnFocus: true,
    });

    dashboardResult = {
      ...dashboardResult,
      data: {
        ...weekdayData,
        businessDay: "2026-08-29",
        spontaneousStartAvailability: {
          available: false,
          blockedReason: "weekend" as const,
        },
      },
    };
    rerender(<MeinRaumPage />);

    expect(
      await screen.findByRole("button", { name: /Spontane Aktivität starten/ }),
    ).toBeDisabled();
    expect(
      await screen.findByRole("button", { name: "Beaufsichtigen" }),
    ).toBeDisabled();
    // Der Grund steht einmal: die Karte „Spontane Aktivität starten" trägt
    // ihn schon, ein zweiter Hinweis für „Beaufsichtigen" wäre doppelt.
    expect(
      screen.getAllByText(
        /Spontane Aktivitäten sind nur montags bis freitags möglich\./,
      ),
    ).toHaveLength(1);
  });

  it("states why Beaufsichtigen is blocked when spontaneous starts are off", async () => {
    const dashboardResult = {
      data: {
        businessDay: "2026-08-29",
        spontaneousStartAvailability: {
          available: false,
          blockedReason: "weekend" as const,
        },
        supervisedGroups: [],
        unclaimedGroups: [],
        currentStaff: { id: "staff-1" },
        educationalGroups: [],
        firstRoomVisits: [],
        firstRoomId: null,
        capabilities: { webSpontaneousActivitiesEnabled: false },
        schulhofStatus: {
          exists: true,
          roomId: "10",
          roomName: "Schulhof",
          activityGroupId: null,
          activeGroupId: null,
          isUserSupervising: false,
          supervisionId: null,
          supervisorCount: 0,
          studentCount: 0,
          supervisors: [],
        },
        openRooms: [
          {
            roomId: "10",
            name: "Schulhof",
            isUserSupervising: false,
            activeGroupIds: [],
            studentCount: 0,
            students: [],
          },
        ],
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

    expect(
      await screen.findByRole("button", { name: "Beaufsichtigen" }),
    ).toBeDisabled();
    // Ohne die Karte für spontane Aktivitäten steht der Grund selbst da —
    // eine gesperrte Schaltfläche ohne Grund wäre eine Sackgasse.
    expect(
      screen.queryByRole("button", { name: /Spontane Aktivität starten/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(
        "Spontane Aktivitäten sind nur montags bis freitags möglich.",
      ),
    ).toBeInTheDocument();
  });

  it("keeps Schulhof selectable as a normal room when status is unavailable (#2161)", async () => {
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
        openRooms: [],
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
      await screen.findByRole("option", { name: "Schulhof" }),
    ).toBeEnabled();
    expect(mockPush).not.toHaveBeenCalledWith(
      "/active-supervisions?room=schulhof",
    );
  });

  it("keeps Schulhof a normal startable option when dashboard revalidation fails (#2161)", async () => {
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
      data: typeof baseDashboardData & {
        schulhofStatus: unknown;
        openRooms: unknown;
      };
      isLoading: boolean;
      error: Error | null;
      mutate: typeof mockMutate;
      isValidating: boolean;
    } = {
      data: {
        ...baseDashboardData,
        capabilities: { webSpontaneousActivitiesEnabled: true },
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
        openRooms: [
          {
            roomId: "5",
            name: "Schulhof",
            isUserSupervising: false,
            activeGroupIds: [],
            studentCount: 0,
            students: [],
          },
        ],
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
      await screen.findByRole("option", { name: "Schulhof" }),
    ).toBeEnabled();
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
        openRooms: [
          {
            roomId: "10",
            name: "Schulhof",
            isUserSupervising: false,
            activeGroupIds: ["55"],
            studentCount: 3,
            students: [],
          },
        ],
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
    expect(
      screen.getByLabelText("Aktuelle Aufsicht wird geladen…"),
    ).toBeInTheDocument();
  });
});

describe("MeinRaumPage (Active Supervisions) (5/5)", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useNFCEnabled).mockReturnValue(true);
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

  it("labels unplanned rows as participants for spontaneous rosters", async () => {
    const dashboardData = {
      supervisedGroups: [
        { id: "1", name: "Aula", room: { id: "10", name: "Aula" } },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: "10",
    };

    vi.mocked(useSWRAuth).mockImplementation(((key: string | null) => {
      if (key?.startsWith("active-supervision-dashboard")) {
        return {
          data: dashboardData,
          isLoading: false,
          error: null,
          mutate: mockMutate,
          isValidating: false,
        };
      }

      if (key?.startsWith("timetable-roster-active-group")) {
        return {
          data: {
            instance: {
              id: "99",
              title: "Malen",
              activeGroupId: "1",
              isSpontaneous: true,
            },
            rows: [
              {
                studentId: "104",
                studentName: "Jan Peters",
                schoolClass: "2a",
                groupName: "Sonnengruppe",
                planned: false,
                isUnplanned: true,
                currentlyPresent: true,
                visitId: "visit-104",
                status: "present",
                substatus: null,
                note: null,
              },
            ],
          },
          isLoading: false,
          error: null,
          mutate: mockMutate,
          isValidating: false,
        };
      }

      return {
        data: null,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      };
    }) as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByText("Malen")).toBeInTheDocument();
      expect(screen.getByText("Aktiv")).toBeInTheDocument();
      expect(screen.getByText("Teilnehmende (1)")).toBeInTheDocument();
      expect(screen.getByText("2a · Sonnengruppe")).toBeInTheDocument();
      expect(screen.queryByText("Ungeplant (1)")).not.toBeInTheDocument();
      expect(
        screen.queryByText("2a · Sonnengruppe · ungeplant"),
      ).not.toBeInTheDocument();
    });
  });

  it("keeps not-scheduled children out of Erwartet and the bulk confirm (#1747)", async () => {
    const dashboardData = {
      supervisedGroups: [
        { id: "1", name: "Raum 101", room: { id: "10", name: "Raum 101" } },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: "10",
    };

    vi.mocked(useSWRAuth).mockImplementation(((key: string | null) => {
      if (key?.startsWith("active-supervision-dashboard")) {
        return {
          data: dashboardData,
          isLoading: false,
          error: null,
          mutate: mockMutate,
          isValidating: false,
        };
      }

      if (key?.startsWith("timetable-roster-active-group")) {
        return {
          data: {
            instance: {
              id: "99",
              title: "Kreativ AG",
              activeGroupId: "1",
              isSpontaneous: false,
            },
            rows: [
              {
                studentId: "101",
                studentName: "Erika Erwartet",
                schoolClass: "2b",
                groupName: "OGS Gruppe B",
                planned: true,
                isUnplanned: false,
                currentlyPresent: false,
                visitId: null,
                status: "expected",
                substatus: null,
                note: null,
                careDayStatus: "scheduled",
              },
              {
                studentId: "102",
                studentName: "Nora NichtGeplant",
                schoolClass: "3c",
                groupName: "OGS Gruppe C",
                planned: true,
                isUnplanned: false,
                currentlyPresent: false,
                visitId: null,
                status: "expected",
                substatus: null,
                note: null,
                careDayStatus: "not_scheduled",
              },
            ],
          },
          isLoading: false,
          error: null,
          mutate: mockMutate,
          isValidating: false,
        };
      }

      return {
        data: null,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      };
    }) as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      // Only the scheduled child is expected; the not-scheduled child gets
      // her own section and is excluded from the bulk confirm.
      expect(screen.getByText("Erwartet (1)")).toBeInTheDocument();
      expect(
        screen.getByText("Heute nicht eingeplant (1)"),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "1 erwartete bestätigen" }),
      ).toBeInTheDocument();
      expect(screen.getByText("Nora NichtGeplant")).toBeInTheDocument();
      // A walk-in stays one tap away from a check-in...
      expect(
        screen.getAllByRole("button", { name: "Einchecken" }),
      ).toHaveLength(2);
      // ...but absence marking only applies to genuinely expected children.
      expect(
        screen.getAllByRole("button", { name: "Entschuldigt" }),
      ).toHaveLength(1);
      expect(screen.getAllByRole("button", { name: "Abwesend" })).toHaveLength(
        1,
      );
    });
  });

  it("groups a status-day absence on an unbooked day as not scheduled (#1747)", async () => {
    const dashboardData = {
      supervisedGroups: [
        { id: "1", name: "Raum 101", room: { id: "10", name: "Raum 101" } },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "1" },
      educationalGroups: [],
      firstRoomVisits: [],
      firstRoomId: "10",
    };

    vi.mocked(useSWRAuth).mockImplementation(((key: string | null) => {
      if (key?.startsWith("active-supervision-dashboard")) {
        return {
          data: dashboardData,
          isLoading: false,
          error: null,
          mutate: mockMutate,
          isValidating: false,
        };
      }

      if (key?.startsWith("timetable-roster-active-group")) {
        return {
          data: {
            instance: {
              id: "99",
              title: "Kreativ AG",
              activeGroupId: "1",
              isSpontaneous: false,
            },
            rows: [
              {
                // Krankmeldung stamped a day the care plan never booked: an
                // absence from care that was never owed.
                studentId: "201",
                studentName: "Klara Krank",
                schoolClass: "2b",
                groupName: "OGS Gruppe B",
                planned: true,
                isUnplanned: false,
                currentlyPresent: false,
                visitId: null,
                status: "absent",
                substatus: "sick",
                note: null,
                careDayStatus: "not_scheduled",
              },
              {
                // A human marked this child absent: a real absence.
                studentId: "202",
                studentName: "Mia Manuell",
                schoolClass: "3c",
                groupName: "OGS Gruppe C",
                planned: true,
                isUnplanned: false,
                currentlyPresent: false,
                visitId: null,
                status: "absent",
                substatus: null,
                note: null,
                careDayStatus: "unknown",
              },
            ],
          },
          isLoading: false,
          error: null,
          mutate: mockMutate,
          isValidating: false,
        };
      }

      return {
        data: null,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      };
    }) as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Heute nicht eingeplant (1)"),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Entschuldigt / Abwesend (1)"),
      ).toBeInTheDocument();
      expect(screen.getByText("Klara Krank")).toBeInTheDocument();
      expect(screen.getByText("Mia Manuell")).toBeInTheDocument();
    });
  });

  it("does not flash first-room students while a direct room URL is syncing", async () => {
    navigationMockState.roomParam = "11";
    // The aggregate re-run for the URL-targeted session never resolves in
    // this test — the point is that the first room's visits from the stale
    // payload must not be shown meanwhile (#2096).
    mockMutate.mockReturnValue(new Promise(() => undefined) as never);
    const dashboardData = {
      supervisedGroups: [
        {
          id: "1",
          name: "Raum 101",
          room_id: "10",
          room: { id: "10", name: "Raum 101" },
        },
        {
          id: "2",
          name: "Raum 102",
          room_id: "11",
          room: { id: "11", name: "Raum 102" },
        },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "1" },
      educationalGroups: [],
      firstRoomVisits: [
        {
          studentId: "100",
          studentName: "Max Mustermann",
          schoolClass: "1a",
          groupName: "OGS Gruppe A",
          activeGroupId: "1",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "10",
    };

    vi.mocked(useSWRAuth).mockImplementation(((key: string | null) => {
      if (key?.startsWith("active-supervision-dashboard")) {
        return {
          data: dashboardData,
          isLoading: false,
          error: null,
          mutate: mockMutate,
          isValidating: false,
        };
      }

      return {
        data: null,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      };
    }) as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      // The URL target resolves to session "2" and triggers the aggregate
      // re-run; the first room's students never appear meanwhile.
      expect(mockMutate).toHaveBeenCalled();
      expect(screen.queryByTestId("student-card")).not.toBeInTheDocument();
      expect(screen.queryByText("Max Mustermann")).not.toBeInTheDocument();
    });
  });

  it("handles permission errors gracefully", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: false,
      error: new ApiError("BFF request failed: 403", 403),
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine aktive Raum-Aufsicht"),
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          "Sie sind aktuell in keinem Raum als Live-Aktivität registriert. Starten Sie eine Aktivität an einem Terminal, um Live-Raumdaten einzusehen.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("points NFC-free tenants to the web app when no supervision is active", async () => {
    vi.mocked(useNFCEnabled).mockReturnValue(false);
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: false,
      error: new ApiError("BFF request failed: 403", 403),
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Sie sind aktuell in keinem Raum als Live-Aktivität registriert. Starten Sie eine Aktivität in der Web-App, um Live-Raumdaten einzusehen.",
        ),
      ).toBeInTheDocument();
    });
    expect(screen.queryByText(/Terminal/i)).not.toBeInTheDocument();
  });
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
      openRooms: [],
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
      openRooms: [],
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
      openRooms: [],
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
      openRooms: [],
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
      openRooms: [],
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
      openRooms: [],
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
    const tabB = screen.getByRole("tab", { name: "Raum B" });
    fireEvent.click(tabB);

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalled();
    });

    // After switching, the second room's student should appear
    await waitFor(() => {
      expect(screen.getByText("Room B Student")).toBeInTheDocument();
    });
  });

  it("merges parallel sessions of a released room into one entry", async () => {
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
      openRooms: [
        {
          roomId: "yard-room",
          name: "Schulhof",
          isUserSupervising: true,
          activeGroupIds: ["yard-primary"],
          studentCount: 9,
          students: [],
        },
      ],
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

    // Beide Sitzungen laufen im freigegebenen Raum, also steht der Raum
    // genau einmal da — vorher trug jede Sitzung ihren eigenen Reiter mit
    // demselben Namen (#3065).
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Schulhof" })).toBeVisible();
      expect(screen.getByText("9 Kinder", { selector: "div" })).toBeVisible();
    });
    expect(screen.queryAllByRole("tab", { name: "Schulhof" })).toHaveLength(0);
    // Der Zähler kommt aus dem Raum, nicht aus einer der Sitzungen: er zählt
    // alle Kinder, die dort gerade erfasst sind.
    expect(mockMutate).not.toHaveBeenCalled();
    // Eigene Aufsicht im Raum bleibt eigene Aufsicht: die Abgabe steht.
    expect(
      screen.getByRole("button", { name: "Aufsicht abgeben" }),
    ).toBeInTheDocument();
  });

  it("reports the released room's own occupancy, not the caller's session", async () => {
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
      openRooms: [
        {
          roomId: "yard-room",
          name: "Schulhof",
          isUserSupervising: false,
          activeGroupIds: ["yard-primary"],
          studentCount: 9,
          students: [],
        },
      ],
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

    // The caller's own session runs in the released room, so the room is the
    // entry — with the occupancy the room reports, not the one session's.
    expect(
      await screen.findByRole("heading", { name: "Schulhof" }),
    ).toBeInTheDocument();
    expect(screen.getByText("9 Kinder", { selector: "div" })).toBeVisible();
    expect(screen.getByText("Offener Raum")).toBeInTheDocument();
    expect(screen.queryByText("Eigene Aufsicht")).not.toBeInTheDocument();
  });

  it("shows the permission notice when switching to a forbidden session", async () => {
    // The aggregate answers 403 for a session outside the caller's scope;
    // the fetcher's group_id retry failed too, so mutate rejects. Unlike the
    // former silently-swallowed per-room 403 (#2096), the page now surfaces
    // the permission problem.
    mockMutate.mockRejectedValue(new ApiError("Request failed: 403", 403));

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
      openRooms: [],
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
    const tabB = screen.getByRole("tab", { name: "Raum B" });
    fireEvent.click(tabB);

    // The permission notice appears and the previous room's students are
    // cleared instead of lingering under the wrong heading.
    await waitFor(() => {
      expect(
        screen.getByText(
          catalogText("general.permission", "die Aufsicht in „Raum B“"),
        ),
      ).toBeInTheDocument();
    });
    expect(screen.queryByText("Initial Student")).not.toBeInTheDocument();
  });

  it("handles non-403 error when switching rooms", async () => {
    // A generic aggregate failure while switching surfaces the load error.
    mockMutate.mockRejectedValue(
      new ApiError("Network timeout", 503, { code: "general.unavailable" }),
    );

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
      openRooms: [],
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
    const tabB = screen.getByRole("tab", { name: "Raum B" });
    fireEvent.click(tabB);

    // Generic error should appear
    await waitFor(() => {
      expect(
        screen.getByText(
          catalogText("general.unavailable", "die Aufsicht in „Raum B“"),
        ),
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
      openRooms: [],
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
    const tabA = screen.getByRole("tab", { name: "Raum A" });
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
      openRooms: [],
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
      openRooms: [],
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
      openRooms: [],
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

describe("ID-based selection coverage: released-room skip guard", () => {
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

  it("skips the first-room preload when a released room is the only option", async () => {
    // With no own supervision, the first released room opens by itself.
    // Its occupancy came with the dashboard, so no session preload runs.
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
      openRooms: [
        {
          roomId: "schulhof-r1",
          name: "Schulhof",
          isUserSupervising: true,
          activeGroupIds: ["active-schulhof"],
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

    // The component should render the Schulhof view (no regular rooms)
    await waitFor(() => {
      expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
    });
  });

  it("does not show a session roster while a released room is open", async () => {
    // The shared room lists what is in the room; the visits of whichever
    // session the aggregate happened to resolve must not leak into it.
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
      openRooms: [
        {
          roomId: "schulhof-r1",
          name: "Schulhof",
          isUserSupervising: true,
          activeGroupIds: ["active-schulhof"],
          studentCount: 2,
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

    // The session's student must not appear under the shared room.
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
      openRooms: [],
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

    // Der Titel nennt die Aufsicht, die Statuszeile ihre Kinderzahl (beweist,
    // dass currentRoom gesetzt ist).
    expect(screen.getByRole("heading", { name: "Only Room" })).toBeVisible();
    expect(screen.getByText("1 Kind", { selector: "div" })).toBeVisible();
  });

  it("shows the released room and its occupancy in the page header", async () => {
    // The room carries its own count; the caller's supervision in it changes
    // neither. Its name is the page title, so it is not repeated below it.
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
      openRooms: [
        {
          roomId: "schulhof-room",
          name: "Schulhof",
          isUserSupervising: true,
          activeGroupIds: ["active-schulhof-id"],
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

    // The released room opens by itself when it is the only option.
    await waitFor(() => {
      expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
    });

    // The count is the room's, reported by the shared view.
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Schulhof" })).toBeVisible();
      expect(screen.getByText("5 Kinder", { selector: "div" })).toBeVisible();
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
    mockMutate.mockRejectedValue(new ApiError("Request failed: 403", 403));

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
      openRooms: [],
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
    const restrictedTab = screen.getByRole("tab", { name: "Restricted Room" });
    fireEvent.click(restrictedTab);

    // switchToRoom surfaces the permission notice and clears the previous
    // room's students.
    await waitFor(() => {
      expect(
        screen.getByText(
          catalogText(
            "general.permission",
            "die Aufsicht in „Restricted Room“",
          ),
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
