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
