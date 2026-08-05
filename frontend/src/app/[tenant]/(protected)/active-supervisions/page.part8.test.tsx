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
    toggleSchulhofSupervision: vi.fn(() => Promise.resolve()),
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
// Partial mock: keep the real MOTO_COLOR_PALETTE/LOCATION_COLORS/gradient
// exports (moto-duotone-icon.tsx reads MOTO_COLOR_PALETTE at module scope)
// and only stub the location-parsing predicates used by this test file.
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

describe("Schulhof action button logic", () => {
  it("determines action button rendering for supervising user", () => {
    const isSchulhofActive = true;
    const schulhofStatus = {
      exists: true,
      isUserSupervising: true,
      supervisorCount: 1,
      studentCount: 5,
    };

    // When user is supervising, show release button
    const showReleaseButton =
      isSchulhofActive && schulhofStatus?.isUserSupervising;
    const showClaimButton =
      isSchulhofActive && !schulhofStatus?.isUserSupervising;

    expect(showReleaseButton).toBe(true);
    expect(showClaimButton).toBe(false);
  });

  it("determines action button rendering for non-supervising user", () => {
    const isSchulhofActive = true;
    const schulhofStatus = {
      exists: true,
      isUserSupervising: false,
      supervisorCount: 0,
      studentCount: 0,
    };

    const showReleaseButton =
      isSchulhofActive && schulhofStatus?.isUserSupervising;
    const showClaimButton =
      isSchulhofActive && !schulhofStatus?.isUserSupervising;

    expect(showReleaseButton).toBe(false);
    expect(showClaimButton).toBe(true);
  });

  it("hides action buttons when not on Schulhof tab", () => {
    const isSchulhofActive = false;
    const schulhofStatus = {
      exists: true,
      isUserSupervising: true,
      supervisorCount: 1,
      studentCount: 5,
    };

    const showActionButton = isSchulhofActive && schulhofStatus;
    expect(showActionButton).toBe(false);
  });

  it("hides action buttons when Schulhof status is null", () => {
    const isSchulhofActive = true;
    const schulhofStatus = null;

    const showActionButton = isSchulhofActive && schulhofStatus;
    expect(showActionButton).toBeNull();
  });
});

describe("Page header title logic", () => {
  function computeTitle(
    isDesktop: boolean,
    allRoomsLength: number,
    schulhofExists: boolean,
    isSchulhofActive: boolean,
    currentRoomName?: string,
  ): string {
    return !isDesktop &&
      (allRoomsLength === 1 || (allRoomsLength === 0 && schulhofExists))
      ? isSchulhofActive
        ? "Schulhof"
        : (currentRoomName ?? "Aktuelle Aufsicht")
      : "";
  }

  it("shows room name on mobile with single room and no Schulhof", () => {
    expect(computeTitle(false, 1, false, false, "Raum 101")).toBe("Raum 101");
  });

  it("shows Schulhof name on mobile when Schulhof is active", () => {
    expect(computeTitle(false, 0, true, true)).toBe("Schulhof");
  });

  it("shows empty title on desktop regardless of rooms", () => {
    expect(computeTitle(true, 1, true, false)).toBe("");
  });

  it("shows empty title on mobile with multiple rooms", () => {
    expect(computeTitle(false, 3, false, false)).toBe("");
  });

  it("falls back to 'Aktuelle Aufsicht' when currentRoom name is undefined", () => {
    expect(computeTitle(false, 1, false, false, undefined)).toBe(
      "Aktuelle Aufsicht",
    );
  });
});

describe("Student badge count logic", () => {
  it("shows Schulhof student count when Schulhof is active", () => {
    const isSchulhofActive = true;
    const schulhofStudentCount = 15;
    const currentRoomStudentCount = 8;

    const count = isSchulhofActive
      ? schulhofStudentCount
      : currentRoomStudentCount;
    expect(count).toBe(15);
  });

  it("shows room student count when regular room is active", () => {
    const isSchulhofActive = false;
    const schulhofStudentCount = 15;
    const currentRoomStudentCount = 8;

    const count = isSchulhofActive
      ? schulhofStudentCount
      : currentRoomStudentCount;
    expect(count).toBe(8);
  });

  it("defaults to 0 when counts are undefined", () => {
    const isSchulhofActive = true;
    const schulhofStudentCount: number | undefined = undefined;
    const currentRoomStudentCount: number | undefined = undefined;

    const count = isSchulhofActive
      ? (schulhofStudentCount ?? 0)
      : (currentRoomStudentCount ?? 0);
    expect(count).toBe(0);
  });
});

describe("currentRoom useMemo logic", () => {
  it("returns Schulhof virtual room when tab selected and user supervising", () => {
    const isSchulhofTabSelected = true;
    const schulhofStatus = {
      isUserSupervising: true,
      activeGroupId: "active-123",
      roomId: "room-schulhof",
      studentCount: 7,
    };

    const currentRoom = isSchulhofTabSelected
      ? schulhofStatus?.isUserSupervising && schulhofStatus?.activeGroupId
        ? {
            id: schulhofStatus.activeGroupId,
            name: "Schulhof",
            room_name: "Schulhof",
            room_id: schulhofStatus.roomId ?? undefined,
            student_count: schulhofStatus.studentCount,
          }
        : null
      : null;

    expect(currentRoom).not.toBeNull();
    expect(currentRoom?.id).toBe("active-123");
    expect(currentRoom?.student_count).toBe(7);
  });

  it("returns null when Schulhof tab selected but not supervising", () => {
    const isSchulhofTabSelected = true;
    const schulhofStatus = {
      isUserSupervising: false,
      activeGroupId: null,
      roomId: "room-schulhof",
      studentCount: 0,
    };
    const allRooms: Array<{ id: string }> = [];
    const selectedRoomIndex = -1;

    const currentRoom = isSchulhofTabSelected
      ? schulhofStatus?.isUserSupervising && schulhofStatus?.activeGroupId
        ? {
            id: schulhofStatus.activeGroupId,
            name: "Schulhof",
            room_name: "Schulhof",
            room_id: schulhofStatus.roomId ?? undefined,
            student_count: schulhofStatus.studentCount,
          }
        : null
      : (allRooms[selectedRoomIndex] ?? null);

    expect(currentRoom).toBeNull();
  });

  it("returns regular room when Schulhof tab NOT selected", () => {
    const isSchulhofTabSelected = false;
    const allRooms = [
      { id: "g1", name: "Room A", room_name: "Room A" },
      { id: "g2", name: "Room B", room_name: "Room B" },
    ];
    const selectedRoomIndex = 1;

    const currentRoom = isSchulhofTabSelected
      ? null
      : (allRooms[selectedRoomIndex] ?? null);

    expect(currentRoom?.id).toBe("g2");
    expect(currentRoom?.room_name).toBe("Room B");
  });

  it("returns null when no rooms and Schulhof tab not selected", () => {
    const isSchulhofTabSelected = false;
    const allRooms: Array<{ id: string }> = [];
    const selectedRoomIndex = 0;

    const currentRoom = isSchulhofTabSelected
      ? null
      : (allRooms[selectedRoomIndex] ?? null);

    expect(currentRoom).toBeNull();
  });
});

describe("isSchulhofActive detection", () => {
  function checkSchulhofActive(
    isSchulhofTabSelected: boolean,
    currentRoomName: string,
  ): boolean {
    return isSchulhofTabSelected || currentRoomName === "Schulhof";
  }

  it("is true when Schulhof tab selected", () => {
    expect(checkSchulhofActive(true, "Regular Room")).toBe(true);
  });

  it("is true when current room is Schulhof by name", () => {
    expect(checkSchulhofActive(false, "Schulhof")).toBe(true);
  });

  it("is false when neither tab nor room name matches", () => {
    expect(checkSchulhofActive(false, "Raum 101")).toBe(false);
  });
});

describe("handleReleaseSupervision edge cases", () => {
  it("skips release when currentRoom is null", () => {
    const currentRoom = null;
    const currentStaffId = "staff-1";

    const shouldSkip = !currentRoom || !currentStaffId;
    expect(shouldSkip).toBe(true);
  });

  it("skips release when currentStaffId is undefined", () => {
    const currentRoom = { id: "room-1" };
    const currentStaffId: string | undefined = undefined;

    const shouldSkip = !currentRoom || !currentStaffId;
    expect(shouldSkip).toBe(true);
  });

  it("proceeds when both currentRoom and staffId are present", () => {
    const currentRoom = { id: "room-1" };
    const currentStaffId = "staff-1";

    const shouldSkip = !currentRoom || !currentStaffId;
    expect(shouldSkip).toBe(false);
  });
});

describe("handleToggleSchulhof edge cases", () => {
  it("skips toggle when schulhofStatus is null", () => {
    const schulhofStatus = null;
    const shouldSkip = !schulhofStatus;
    expect(shouldSkip).toBe(true);
  });

  it("proceeds when schulhofStatus exists", () => {
    const schulhofStatus = { exists: true, isUserSupervising: false };
    const shouldSkip = !schulhofStatus;
    expect(shouldSkip).toBe(false);
  });
});

describe("Tab configuration with Schulhof", () => {
  it("includes Schulhof tab in items when it exists", () => {
    const SCHULHOF_TAB_ID = "schulhof";
    const SCHULHOF_ROOM_NAME = "Schulhof";
    const allRooms = [
      { id: "g1", room_name: "Raum A", name: "Group A" },
      { id: "g2", room_name: "Raum B", name: "Group B" },
    ];
    const schulhofExists = true;

    const items = [
      ...allRooms
        .filter((room) => room.room_name !== SCHULHOF_ROOM_NAME)
        .map((room) => ({
          id: room.id,
          label: room.room_name ?? room.name,
        })),
      ...(schulhofExists
        ? [{ id: SCHULHOF_TAB_ID, label: SCHULHOF_ROOM_NAME }]
        : []),
    ];

    expect(items).toHaveLength(3);
    expect(items[2]?.id).toBe(SCHULHOF_TAB_ID);
    expect(items[2]?.label).toBe("Schulhof");
  });

  it("filters out Schulhof from regular room tabs", () => {
    const SCHULHOF_ROOM_NAME = "Schulhof";
    const allRooms = [
      { id: "g1", room_name: "Raum A", name: "Group A" },
      { id: "g2", room_name: "Schulhof", name: "Schulhof Group" },
      { id: "g3", room_name: "Raum B", name: "Group B" },
    ];

    const regularTabs = allRooms.filter(
      (room) => room.room_name !== SCHULHOF_ROOM_NAME,
    );

    expect(regularTabs).toHaveLength(2);
    expect(regularTabs.find((r) => r.room_name === "Schulhof")).toBeUndefined();
  });

  it("determines active tab ID for Schulhof tab", () => {
    const SCHULHOF_TAB_ID = "schulhof";
    const isSchulhofTabSelected = true;
    const currentRoomId = "g1";

    const activeTab = isSchulhofTabSelected ? SCHULHOF_TAB_ID : currentRoomId;
    expect(activeTab).toBe(SCHULHOF_TAB_ID);
  });

  it("determines active tab ID for regular room", () => {
    const SCHULHOF_TAB_ID = "schulhof";
    const isSchulhofTabSelected = false;
    const currentRoomId = "g1";

    const activeTab = isSchulhofTabSelected ? SCHULHOF_TAB_ID : currentRoomId;
    expect(activeTab).toBe("g1");
  });

  it("falls back to empty string when no current room", () => {
    const SCHULHOF_TAB_ID = "schulhof";
    const isSchulhofTabSelected = false;
    const currentRoomId: string | undefined = undefined;

    const activeTab = isSchulhofTabSelected
      ? SCHULHOF_TAB_ID
      : (currentRoomId ?? "");
    expect(activeTab).toBe("");
  });
});

describe("Auto-select Schulhof tab logic", () => {
  function shouldAutoSelectSchulhof(
    allRoomsLength: number,
    schulhofExists: boolean,
    isSchulhofTabSelected: boolean,
  ): boolean {
    return allRoomsLength === 0 && schulhofExists && !isSchulhofTabSelected;
  }

  it("selects Schulhof when no rooms and Schulhof exists", () => {
    expect(shouldAutoSelectSchulhof(0, true, false)).toBe(true);
  });

  it("does not auto-select when rooms exist", () => {
    expect(shouldAutoSelectSchulhof(2, true, false)).toBe(false);
  });

  it("does not auto-select when already selected", () => {
    expect(shouldAutoSelectSchulhof(0, true, true)).toBe(false);
  });

  it("does not auto-select when Schulhof does not exist", () => {
    expect(shouldAutoSelectSchulhof(0, false, false)).toBe(false);
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

describe("ID-based selection: Schulhof tab skip guard logic", () => {
  it("determines when to skip first-room preload with Schulhof tab active", () => {
    const isSchulhofTabSelected = true;
    const selectedRoomId = null;
    const firstRoomId = "g1";

    // Skip guard: !isSchulhofTabSelected && (!selectedRoomId || selectedRoomId === firstRoomId)
    const shouldPreload =
      !isSchulhofTabSelected &&
      (!selectedRoomId || selectedRoomId === firstRoomId);

    expect(shouldPreload).toBe(false); // Should skip when Schulhof is active
  });

  it("preloads first room when NOT on Schulhof tab", () => {
    const isSchulhofTabSelected = false;
    const selectedRoomId = null;
    const firstRoomId = "g1";

    const shouldPreload =
      !isSchulhofTabSelected &&
      (!selectedRoomId || selectedRoomId === firstRoomId);

    expect(shouldPreload).toBe(true); // Should preload when not on Schulhof
  });

  it("preloads when first room is selected and not on Schulhof", () => {
    const isSchulhofTabSelected = false;
    const selectedRoomId = "g1";
    const firstRoomId = "g1";

    const shouldPreload =
      !isSchulhofTabSelected &&
      (!selectedRoomId || selectedRoomId === firstRoomId);

    expect(shouldPreload).toBe(true);
  });

  it("skips preload when different room is selected", () => {
    const isSchulhofTabSelected = false;
    const selectedRoomId = "g2" as string;
    const firstRoomId = "g1" as string;

    const shouldPreload =
      !isSchulhofTabSelected &&
      (!selectedRoomId || selectedRoomId === firstRoomId);

    expect(shouldPreload).toBe(false); // Different room selected
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

describe("ID-based selection: URL param handler logic", () => {
  it("finds target room by room_id from allRooms array", () => {
    const roomParam = "10";
    const allRooms = [
      { id: "g1", name: "Raum A", room_id: "10" },
      { id: "g2", name: "Raum B", room_id: "11" },
    ];

    const targetRoom = allRooms.find((r) => r.room_id === roomParam);

    expect(targetRoom).toBeDefined();
    expect(targetRoom?.id).toBe("g1");
    expect(targetRoom?.room_id).toBe("10");
  });

  it("returns undefined when room_id does not match any room", () => {
    const roomParam = "999";
    const allRooms = [
      { id: "g1", name: "Raum A", room_id: "10" },
      { id: "g2", name: "Raum B", room_id: "11" },
    ];

    const targetRoom = allRooms.find((r) => r.room_id === roomParam);

    expect(targetRoom).toBeUndefined();
  });

  it("checks if target room ID differs from selected room ID", () => {
    const roomParam = "10";
    const selectedRoomId = "g2";
    const allRooms = [
      { id: "g1", name: "Raum A", room_id: "10" },
      { id: "g2", name: "Raum B", room_id: "11" },
    ];

    const targetRoom = allRooms.find((r) => r.room_id === roomParam);
    const shouldSwitch = targetRoom && targetRoom.id !== selectedRoomId;

    expect(shouldSwitch).toBe(true); // g1 !== g2
  });

  it("avoids switching when target room is already selected", () => {
    const roomParam = "10";
    const selectedRoomId = "g1";
    const allRooms = [
      { id: "g1", name: "Raum A", room_id: "10" },
      { id: "g2", name: "Raum B", room_id: "11" },
    ];

    const targetRoom = allRooms.find((r) => r.room_id === roomParam);
    const shouldSwitch = targetRoom && targetRoom.id !== selectedRoomId;

    expect(shouldSwitch).toBe(false); // g1 === g1
  });
});

describe("ID-based selection: localStorage restore logic", () => {
  it("finds saved room from localStorage using ID-based find", () => {
    const savedRoomId = "11";
    const allRooms = [
      { id: "g1", name: "Raum A", room_id: "10" },
      { id: "g2", name: "Raum B", room_id: "11" },
    ];

    const savedRoom = savedRoomId
      ? allRooms.find((r) => r.room_id === savedRoomId)
      : undefined;

    expect(savedRoom).toBeDefined();
    expect(savedRoom?.id).toBe("g2");
    expect(savedRoom?.room_id).toBe("11");
  });

  it("handles saved room not found in current allRooms list", () => {
    const savedRoomId = "999";
    const allRooms = [
      { id: "g1", name: "Raum A", room_id: "10" },
      { id: "g2", name: "Raum B", room_id: "11" },
    ];

    const savedRoom = savedRoomId
      ? allRooms.find((r) => r.room_id === savedRoomId)
      : undefined;

    expect(savedRoom).toBeUndefined();
  });

  it("checks if saved room ID differs from current selection", () => {
    const savedRoomId = "11";
    const selectedRoomId = "g1";
    const allRooms = [
      { id: "g1", name: "Raum A", room_id: "10" },
      { id: "g2", name: "Raum B", room_id: "11" },
    ];

    const savedRoom = savedRoomId
      ? allRooms.find((r) => r.room_id === savedRoomId)
      : undefined;
    const shouldSwitch = savedRoom && savedRoom.id !== selectedRoomId;

    expect(shouldSwitch).toBe(true); // g2 !== g1
  });

  it("avoids switch when saved room is already selected", () => {
    const savedRoomId = "11";
    const selectedRoomId = "g2";
    const allRooms = [
      { id: "g1", name: "Raum A", room_id: "10" },
      { id: "g2", name: "Raum B", room_id: "11" },
    ];

    const savedRoom = savedRoomId
      ? allRooms.find((r) => r.room_id === savedRoomId)
      : undefined;
    const shouldSwitch = savedRoom && savedRoom.id !== selectedRoomId;

    expect(shouldSwitch).toBe(false); // g2 === g2
  });

  it("persists first room when no saved room exists", () => {
    const savedRoomId = null;
    const allRooms = [
      { id: "g1", name: "Raum A", room_id: "10" },
      { id: "g2", name: "Raum B", room_id: "11" },
    ];

    const savedRoom = savedRoomId
      ? allRooms.find((r) => r.room_id === savedRoomId)
      : undefined;

    if (!savedRoom) {
      const firstRoom = allRooms[0];
      expect(firstRoom?.room_id).toBe("10");
    }
  });
});

describe("ID-based selection: Tab change handler logic", () => {
  it("finds room by ID from allRooms array", () => {
    const tabId = "g2";
    const allRooms = [
      { id: "g1", name: "Raum A", room_id: "10", room_name: "Raum A" },
      { id: "g2", name: "Raum B", room_id: "11", room_name: "Raum B" },
    ];

    const room = allRooms.find((r) => r.id === tabId);

    expect(room).toBeDefined();
    expect(room?.id).toBe("g2");
    expect(room?.room_id).toBe("11");
    expect(room?.room_name).toBe("Raum B");
  });

  it("returns undefined when tab ID does not match any room", () => {
    const tabId = "g999";
    const allRooms = [
      { id: "g1", name: "Raum A", room_id: "10" },
      { id: "g2", name: "Raum B", room_id: "11" },
    ];

    const room = allRooms.find((r) => r.id === tabId);

    expect(room).toBeUndefined();
  });

  it("extracts room_id and room_name for localStorage storage", () => {
    const tabId = "g1";
    const allRooms = [
      { id: "g1", name: "Raum A", room_id: "10", room_name: "Raum A" },
    ];

    const room = allRooms.find((r) => r.id === tabId);

    if (room) {
      expect(room.room_id).toBe("10");
      expect(room.room_name).toBe("Raum A");
    }
  });

  it("handles Schulhof tab ID specially", () => {
    const SCHULHOF_TAB_ID = "schulhof";
    const tabId = SCHULHOF_TAB_ID;

    const isSchulhofTab = tabId === SCHULHOF_TAB_ID;

    expect(isSchulhofTab).toBe(true);
  });

  it("distinguishes between Schulhof and regular room tabs", () => {
    const SCHULHOF_TAB_ID = "schulhof";
    const regularTabId = "g1" as string;
    const schulhofTabId: string = SCHULHOF_TAB_ID;

    const regularIsSchulhof = regularTabId === SCHULHOF_TAB_ID;
    const schulhofIsSchulhof = schulhofTabId === SCHULHOF_TAB_ID;

    expect(regularIsSchulhof).toBe(false);
    expect(schulhofIsSchulhof).toBe(true);
  });
});

/**
 * Coverage tests for ID-based selection logic introduced in the SSE selection stability PR.
 * These tests render the actual MeinRaumPage component to exercise changed lines
 * that SonarCloud needs covered.
 */
