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
import { render, screen, waitFor, cleanup } from "@testing-library/react";
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
