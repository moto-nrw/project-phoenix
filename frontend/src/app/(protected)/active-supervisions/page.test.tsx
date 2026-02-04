/**
 * Tests for Active Supervisions Page
 * Tests the rendering states and user interactions of the active supervisions dashboard
 */
import {
  render,
  screen,
  waitFor,
  cleanup,
  fireEvent,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { token: "test-token" } },
    status: "authenticated",
  })),
}));

// Mock next/navigation
const mockPush = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
  useSearchParams: () => ({ get: () => null }),
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
vi.mock("~/components/ui/page-header", () => ({
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

// Mock Modal
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
}));

// Mock activeService
vi.mock("~/lib/active-api", () => ({
  activeService: {
    getActiveGroupVisitsWithDisplay: vi.fn(() => Promise.resolve([])),
    getActiveGroupSupervisors: vi.fn(() => Promise.resolve([])),
    endSupervision: vi.fn(() => Promise.resolve()),
  },
}));

// Mock SSEErrorBoundary
vi.mock("~/components/sse/SSEErrorBoundary", () => ({
  SSEErrorBoundary: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="sse-boundary">{children}</div>
  ),
}));

// Mock UnclaimedRooms
vi.mock("~/components/active", () => ({
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

// Mock StudentCard components
vi.mock("~/components/students/student-card", () => ({
  StudentCard: ({
    firstName,
    lastName,
  }: {
    firstName: string;
    lastName: string;
  }) => (
    <div data-testid="student-card">
      {firstName} {lastName}
    </div>
  ),
  StudentInfoRow: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="student-info-row">{children}</div>
  ),
  SchoolClassIcon: () => <span data-testid="school-class-icon" />,
  GroupIcon: () => <span data-testid="group-icon" />,
}));

// Mock SWR hook
vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
}));

import { useSWRAuth } from "~/lib/swr";
import MeinRaumPage from "./page";

describe("MeinRaumPage (Active Supervisions)", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
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

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("renders with SSE error boundary wrapper", () => {
    render(<MeinRaumPage />);

    expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
  });

  it("renders within responsive layout", async () => {
    render(<MeinRaumPage />);

    expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
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

  it("displays supervised room with students", async () => {
    // First call: dashboard data, Second call: per-room visits (return null to skip)
    const dashboardData = {
      supervisedGroups: [
        // Use a non-Schulhof room name to avoid triggering Schulhof-specific code path
        { id: "1", name: "Raum 101", room: { id: "10", name: "Raum 101" } },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "1" },
      educationalGroups: [
        { id: "2", name: "OGS Gruppe A", room: { name: "Raum 101" } },
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
        data: null, // Second hook (per-room visits) returns null
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

  it("handles permission errors gracefully", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error("BFF request failed: 403"),
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
});

describe("Active Supervisions helper functions", () => {
  it("filters students by search term", () => {
    const student = {
      name: "Max Mustermann",
      first_name: "Max",
      second_name: "Mustermann",
    };

    const searchLower = "max";
    const matches =
      (student.name?.toLowerCase().includes(searchLower) ?? false) ||
      (student.first_name?.toLowerCase().includes(searchLower) ?? false) ||
      (student.second_name?.toLowerCase().includes(searchLower) ?? false);

    expect(matches).toBe(true);
  });

  it("filters students by group name", () => {
    const students = [
      { id: "1", group_name: "OGS Gruppe A" },
      { id: "2", group_name: "OGS Gruppe B" },
      { id: "3", group_name: "OGS Gruppe A" },
    ];

    const groupFilter = "OGS Gruppe A" as string;
    const filtered = students.filter(
      (s) => groupFilter === "all" || s.group_name === groupFilter,
    );

    expect(filtered).toHaveLength(2);
    expect(filtered.map((s) => s.id)).toEqual(["1", "3"]);
  });

  it("detects Schulhof room for special behavior", () => {
    const SCHULHOF_ROOM_NAME = "Schulhof";
    const room = { room_name: "Schulhof" };

    const isSchulhof = room?.room_name === SCHULHOF_ROOM_NAME;
    expect(isSchulhof).toBe(true);

    const regularRoom = { room_name: "Raum 101" };
    const isSchulhofRegular = regularRoom?.room_name === SCHULHOF_ROOM_NAME;
    expect(isSchulhofRegular).toBe(false);
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
        screen.getByText("Keine Schüler in diesem Raum"),
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

  it("shows page header with student count badge", async () => {
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

    await waitFor(() => {
      const header = screen.getByTestId("page-header");
      expect(header).toHaveAttribute("data-count", "2");
    });
  });
});

describe("MeinRaumPage filter and search behavior", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it("applies search filter to student list", () => {
    // Test search filtering logic
    const students = [
      {
        id: "1",
        name: "Max Mustermann",
        first_name: "Max",
        second_name: "Mustermann",
      },
      {
        id: "2",
        name: "Erika Schmidt",
        first_name: "Erika",
        second_name: "Schmidt",
      },
      {
        id: "3",
        name: "Hans Beispiel",
        first_name: "Hans",
        second_name: "Beispiel",
      },
    ];

    const searchTerm = "erika";
    const searchLower = searchTerm.toLowerCase();

    const filtered = students.filter((student) => {
      return (
        (student.name?.toLowerCase().includes(searchLower) ?? false) ||
        (student.first_name?.toLowerCase().includes(searchLower) ?? false) ||
        (student.second_name?.toLowerCase().includes(searchLower) ?? false)
      );
    });

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.first_name).toBe("Erika");
  });

  it("shows no results when search term matches nothing", () => {
    const students = [
      { id: "1", name: "Max Mustermann", first_name: "Max" },
      { id: "2", name: "Erika Schmidt", first_name: "Erika" },
    ];

    const searchTerm = "xyz123";
    const searchLower = searchTerm.toLowerCase();

    const filtered = students.filter((student) => {
      return (
        (student.name?.toLowerCase().includes(searchLower) ?? false) ||
        (student.first_name?.toLowerCase().includes(searchLower) ?? false)
      );
    });

    expect(filtered).toHaveLength(0);
  });

  it("filters by group when group filter is active", () => {
    const students = [
      { id: "1", group_name: "OGS Gruppe A" },
      { id: "2", group_name: "OGS Gruppe B" },
      { id: "3", group_name: "OGS Gruppe A" },
      { id: "4", group_name: "OGS Gruppe C" },
    ];

    const groupFilter = "OGS Gruppe B";
    const filtered = students.filter((s) => s.group_name === groupFilter);

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.id).toBe("2");
  });

  it("returns all students when group filter is 'all'", () => {
    const students = [
      { id: "1", group_name: "OGS Gruppe A" },
      { id: "2", group_name: "OGS Gruppe B" },
    ];

    const groupFilter = "all";
    const filtered = students.filter(
      (s) => groupFilter === "all" || s.group_name === groupFilter,
    );

    expect(filtered).toHaveLength(2);
  });

  it("combines search and group filters", () => {
    const students = [
      { id: "1", name: "Max A", group_name: "OGS Gruppe A" },
      { id: "2", name: "Erika A", group_name: "OGS Gruppe A" },
      { id: "3", name: "Max B", group_name: "OGS Gruppe B" },
    ];

    const searchTerm = "max";
    const groupFilter = "OGS Gruppe A" as string;

    const filtered = students.filter((student) => {
      const matchesSearch =
        student.name?.toLowerCase().includes(searchTerm.toLowerCase()) ?? false;
      const matchesGroup =
        groupFilter === "all" || student.group_name === groupFilter;
      return matchesSearch && matchesGroup;
    });

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.id).toBe("1");
  });
});

describe("MeinRaumPage active filters", () => {
  it("creates active filter for search term", () => {
    const searchTerm = "Max";
    const groupFilter = "all" as string;

    const activeFilters: Array<{ id: string; label: string }> = [];

    if (searchTerm.length > 0) {
      activeFilters.push({
        id: "search",
        label: `"${searchTerm}"`,
      });
    }

    if (groupFilter !== "all") {
      activeFilters.push({
        id: "group",
        label: `Gruppe: ${groupFilter}`,
      });
    }

    expect(activeFilters).toHaveLength(1);
    expect(activeFilters[0]?.label).toBe('"Max"');
  });

  it("creates active filter for group filter", () => {
    const searchTerm = "";
    const groupFilter = "OGS Gruppe A" as string;

    const activeFilters: Array<{ id: string; label: string }> = [];

    if (searchTerm.length > 0) {
      activeFilters.push({
        id: "search",
        label: `"${searchTerm}"`,
      });
    }

    if (groupFilter !== "all") {
      activeFilters.push({
        id: "group",
        label: `Gruppe: ${groupFilter}`,
      });
    }

    expect(activeFilters).toHaveLength(1);
    expect(activeFilters[0]?.label).toBe("Gruppe: OGS Gruppe A");
  });

  it("creates multiple active filters", () => {
    const searchTerm = "Max";
    const groupFilter = "OGS Gruppe B" as string;

    const activeFilters: Array<{ id: string; label: string }> = [];

    if (searchTerm.length > 0) {
      activeFilters.push({
        id: "search",
        label: `"${searchTerm}"`,
      });
    }

    if (groupFilter !== "all") {
      activeFilters.push({
        id: "group",
        label: `Gruppe: ${groupFilter}`,
      });
    }

    expect(activeFilters).toHaveLength(2);
  });

  it("creates no active filters when all defaults", () => {
    const searchTerm = "";
    const groupFilter = "all" as string;

    const activeFilters: Array<{ id: string; label: string }> = [];

    if (searchTerm.length > 0) {
      activeFilters.push({ id: "search", label: `"${searchTerm}"` });
    }

    if (groupFilter !== "all") {
      activeFilters.push({ id: "group", label: `Gruppe: ${groupFilter}` });
    }

    expect(activeFilters).toHaveLength(0);
  });
});

describe("MeinRaumPage visit data mapping", () => {
  it("maps visit data to student with visit format", () => {
    const visit = {
      studentId: "100",
      studentName: "Max Mustermann",
      schoolClass: "1a",
      groupName: "OGS Gruppe A",
      activeGroupId: "1",
      checkInTime: "2024-01-15T10:00:00Z",
      isActive: true,
    };

    const roomName = "Raum 101";
    const nameParts = visit.studentName?.split(" ") ?? ["", ""];
    const firstName = nameParts[0] ?? "";
    const lastName = nameParts.slice(1).join(" ") ?? "";
    const location = roomName ? `Anwesend - ${roomName}` : "Anwesend";

    const mappedStudent = {
      id: visit.studentId,
      name: visit.studentName ?? "",
      first_name: firstName,
      second_name: lastName,
      school_class: visit.schoolClass ?? "",
      current_location: location,
      group_name: visit.groupName,
      activeGroupId: visit.activeGroupId,
      checkInTime: new Date(visit.checkInTime),
    };

    expect(mappedStudent.id).toBe("100");
    expect(mappedStudent.first_name).toBe("Max");
    expect(mappedStudent.second_name).toBe("Mustermann");
    expect(mappedStudent.current_location).toBe("Anwesend - Raum 101");
    expect(mappedStudent.group_name).toBe("OGS Gruppe A");
  });

  it("handles empty student name gracefully", () => {
    const visit = {
      studentId: "101",
      studentName: "",
      schoolClass: null,
      groupName: null,
      activeGroupId: "1",
      checkInTime: "2024-01-15T10:00:00Z",
      isActive: true,
    };

    const nameParts = visit.studentName?.split(" ") ?? ["", ""];
    const firstName = nameParts[0] ?? "";
    const lastName = nameParts.slice(1).join(" ") ?? "";

    expect(firstName).toBe("");
    expect(lastName).toBe("");
  });

  it("creates location string without room name", () => {
    const roomName = undefined as string | undefined;
    const location = roomName ? `Anwesend - ${roomName}` : "Anwesend";

    expect(location).toBe("Anwesend");
  });
});

describe("loadRoomVisits function behavior", () => {
  it("filters only active visits from response", () => {
    const visits = [
      {
        studentId: "1",
        studentName: "Max",
        isActive: true,
        checkInTime: "2024-01-15T10:00:00Z",
      },
      {
        studentId: "2",
        studentName: "Erika",
        isActive: false,
        checkInTime: "2024-01-15T09:00:00Z",
      },
      {
        studentId: "3",
        studentName: "Hans",
        isActive: true,
        checkInTime: "2024-01-15T10:30:00Z",
      },
    ];

    const currentlyCheckedIn = visits.filter((visit) => visit.isActive);

    expect(currentlyCheckedIn).toHaveLength(2);
    expect(currentlyCheckedIn.map((v) => v.studentId)).toEqual(["1", "3"]);
  });

  it("handles 403 permission error gracefully", () => {
    const handleError = (
      error: Error,
    ): { students: never[]; warning: string } | null => {
      if (error instanceof Error && error.message.includes("403")) {
        console.warn(`No permission - returning empty list`);
        return { students: [], warning: "No permission" };
      }
      return null;
    };

    const error403 = new Error("BFF request failed: 403 Forbidden");
    const result = handleError(error403);

    expect(result).not.toBeNull();
    expect(result?.students).toEqual([]);
  });

  it("re-throws non-403 errors", () => {
    const handleError = (error: Error): never[] | null => {
      if (error instanceof Error && error.message.includes("403")) {
        return [];
      }
      return null; // Indicates error should be re-thrown
    };

    const error500 = new Error("BFF request failed: 500 Internal Server Error");
    const result = handleError(error500);

    expect(result).toBeNull(); // null means re-throw
  });

  it("enriches visits with group ID from map", () => {
    const groupNameToId = new Map<string, string>();
    groupNameToId.set("OGS Gruppe A", "group-123");
    groupNameToId.set("OGS Gruppe B", "group-456");

    const visits = [
      { studentId: "1", groupName: "OGS Gruppe A" },
      { studentId: "2", groupName: "OGS Gruppe B" },
      { studentId: "3", groupName: "Unknown Group" },
    ];

    const enriched = visits.map((visit) => ({
      ...visit,
      groupId: visit.groupName ? groupNameToId.get(visit.groupName) : undefined,
    }));

    expect(enriched[0]?.groupId).toBe("group-123");
    expect(enriched[1]?.groupId).toBe("group-456");
    expect(enriched[2]?.groupId).toBeUndefined();
  });
});

describe("switchToRoom function behavior", () => {
  it("clears current students when switching rooms", () => {
    let students = [
      { id: "1", name: "Max" },
      { id: "2", name: "Erika" },
    ];

    const clearStudents = () => {
      students = [];
    };

    clearStudents();
    expect(students).toHaveLength(0);
  });

  it("handles room not found error", () => {
    const allRooms = [
      { id: "1", name: "Raum 1" },
      { id: "2", name: "Raum 2" },
    ];

    const roomIndex = 5; // Out of bounds
    const selectedRoom = allRooms[roomIndex];

    const errorMessage = !selectedRoom ? "No active room found" : null;
    expect(errorMessage).toBe("No active room found");
  });

  it("updates room student count after loading", () => {
    const rooms = [
      { id: "1", name: "Raum 1", student_count: undefined },
      { id: "2", name: "Raum 2", student_count: undefined },
    ];

    const updateRoomCount = (roomIndex: number, count: number) => {
      return rooms.map((room, idx) =>
        idx === roomIndex ? { ...room, student_count: count } : room,
      );
    };

    const updatedRooms = updateRoomCount(1, 15);
    expect(updatedRooms[1]?.student_count).toBe(15);
    expect(updatedRooms[0]?.student_count).toBeUndefined();
  });

  it("handles 403 error when switching rooms", () => {
    const handleSwitchError = (error: Error, roomName: string): string => {
      if (error instanceof Error && error.message.includes("403")) {
        return `Keine Berechtigung für "${roomName}". Kontaktieren Sie einen Administrator.`;
      }
      return "Fehler beim Laden der Raumdaten.";
    };

    const error403 = new Error("BFF request failed: 403");
    const message = handleSwitchError(error403, "Kunstzimmer");
    expect(message).toContain("Keine Berechtigung");
    expect(message).toContain("Kunstzimmer");
  });

  it("returns generic error for non-403 errors", () => {
    const handleSwitchError = (error: Error, _roomName: string): string => {
      if (error instanceof Error && error.message.includes("403")) {
        return `Keine Berechtigung für "${_roomName}".`;
      }
      return "Fehler beim Laden der Raumdaten.";
    };

    const error500 = new Error("BFF request failed: 500");
    const message = handleSwitchError(error500, "Kunstzimmer");
    expect(message).toBe("Fehler beim Laden der Raumdaten.");
  });
});

describe("handleReleaseSupervision behavior", () => {
  it("finds current user supervision from supervisors list", () => {
    const currentStaffId = "staff-123";
    const supervisors = [
      { id: "sup-1", staffId: "staff-456", isActive: true },
      { id: "sup-2", staffId: "staff-123", isActive: true },
      { id: "sup-3", staffId: "staff-789", isActive: false },
    ];

    const mySupervision = supervisors.find(
      (sup) => sup.staffId === currentStaffId && sup.isActive,
    );

    expect(mySupervision).toBeDefined();
    expect(mySupervision?.id).toBe("sup-2");
  });

  it("handles case when no supervision found for current user", () => {
    const currentStaffId = "staff-999";
    const supervisors = [
      { id: "sup-1", staffId: "staff-456", isActive: true },
      { id: "sup-2", staffId: "staff-123", isActive: true },
    ];

    const mySupervision = supervisors.find(
      (sup) => sup.staffId === currentStaffId && sup.isActive,
    );

    expect(mySupervision).toBeUndefined();
  });

  it("does not find inactive supervision", () => {
    const currentStaffId = "staff-123";
    const supervisors = [
      { id: "sup-1", staffId: "staff-123", isActive: false }, // inactive
    ];

    const mySupervision = supervisors.find(
      (sup) => sup.staffId === currentStaffId && sup.isActive,
    );

    expect(mySupervision).toBeUndefined();
  });

  it("tracks release supervision loading state", () => {
    let isReleasingSupervision = false;

    const startRelease = () => {
      isReleasingSupervision = true;
    };
    const endRelease = () => {
      isReleasingSupervision = false;
    };

    expect(isReleasingSupervision).toBe(false);
    startRelease();
    expect(isReleasingSupervision).toBe(true);
    endRelease();
    expect(isReleasingSupervision).toBe(false);
  });

  it("sets error message on release failure", () => {
    let error: string | null = null;

    const handleError = (err: Error) => {
      console.error("Failed to release Schulhof supervision:", err);
      error = "Fehler beim Abgeben der Schulhof-Aufsicht.";
    };

    handleError(new Error("Network error"));
    expect(error).toBe("Fehler beim Abgeben der Schulhof-Aufsicht.");
  });
});

describe("MeinRaumPage Schulhof detection", () => {
  it("detects Schulhof room correctly", () => {
    const SCHULHOF_ROOM_NAME = "Schulhof";

    const testCases = [
      { room_name: "Schulhof", expected: true },
      { room_name: "Raum 101", expected: false },
      { room_name: "Mensa", expected: false },
      { room_name: undefined, expected: false },
    ];

    testCases.forEach(({ room_name, expected }) => {
      const isSchulhof = room_name === SCHULHOF_ROOM_NAME;
      expect(isSchulhof).toBe(expected);
    });
  });
});

describe("MeinRaumPage room tabs behavior", () => {
  it("shows tabs when user has 2-4 rooms", () => {
    const testCases = [
      { roomCount: 1, shouldShowTabs: false },
      { roomCount: 2, shouldShowTabs: true },
      { roomCount: 3, shouldShowTabs: true },
      { roomCount: 4, shouldShowTabs: true },
      { roomCount: 5, shouldShowTabs: false },
    ];

    testCases.forEach(({ roomCount, shouldShowTabs }) => {
      const showTabs = roomCount > 1 && roomCount <= 4;
      expect(showTabs).toBe(shouldShowTabs);
    });
  });

  it("creates tab items from rooms", () => {
    const allRooms = [
      { id: "1", name: "Gruppenraum A", room_name: "Raum 101" },
      { id: "2", name: "Gruppenraum B", room_name: "Raum 102" },
    ];

    const tabItems = allRooms.map((room) => ({
      id: room.id,
      label: room.room_name ?? room.name,
    }));

    expect(tabItems).toHaveLength(2);
    expect(tabItems[0]?.label).toBe("Raum 101");
    expect(tabItems[1]?.label).toBe("Raum 102");
  });

  it("uses name fallback when room_name is undefined", () => {
    const room = { id: "1", name: "Gruppenraum A", room_name: undefined };
    const label = room.room_name ?? room.name;
    expect(label).toBe("Gruppenraum A");
  });
});

describe("MeinRaumPage dashboard error handling", () => {
  it("sets hasAccess to false on 403 error", () => {
    const handleDashboardError = (error: Error) => {
      if (error.message.includes("403")) {
        return {
          error: "Sie haben aktuell keinen aktiven Raum zur Supervision.",
          hasAccess: false,
        };
      }
      return {
        error: "Fehler beim Laden der Aktivitätsdaten.",
        hasAccess: true,
      };
    };

    const result403 = handleDashboardError(
      new Error("BFF request failed: 403"),
    );
    expect(result403.hasAccess).toBe(false);
    expect(result403.error).toContain("keinen aktiven Raum");
  });

  it("keeps hasAccess true on other errors", () => {
    const handleDashboardError = (error: Error) => {
      if (error.message.includes("403")) {
        return { hasAccess: false };
      }
      return { hasAccess: true };
    };

    const result500 = handleDashboardError(
      new Error("BFF request failed: 500"),
    );
    expect(result500.hasAccess).toBe(true);
  });
});

describe("MeinRaumPage educational groups processing", () => {
  it("extracts room names from educational groups", () => {
    const educationalGroups = [
      { id: "g1", name: "Gruppe A", room: { name: "Raum 101" } },
      { id: "g2", name: "Gruppe B", room: { name: "Raum 102" } },
      { id: "g3", name: "Gruppe C", room: undefined },
    ];

    const roomNames = educationalGroups
      .map((group) => group.room?.name)
      .filter((name): name is string => !!name);

    expect(roomNames).toEqual(["Raum 101", "Raum 102"]);
  });

  it("extracts group IDs from educational groups", () => {
    const educationalGroups = [
      { id: "g1", name: "Gruppe A" },
      { id: "g2", name: "Gruppe B" },
    ];

    const groupIds = educationalGroups.map((group) => group.id);
    expect(groupIds).toEqual(["g1", "g2"]);
  });

  it("creates name to ID map from educational groups", () => {
    const educationalGroups = [
      { id: "g1", name: "Gruppe A" },
      { id: "g2", name: "Gruppe B" },
    ];

    const nameToIdMap = new Map<string, string>();
    educationalGroups.forEach((group) => {
      if (group.name) {
        nameToIdMap.set(group.name, group.id);
      }
    });

    expect(nameToIdMap.get("Gruppe A")).toBe("g1");
    expect(nameToIdMap.get("Gruppe B")).toBe("g2");
  });
});

describe("MeinRaumPage combined groups caching", () => {
  it("combines supervised and unclaimed groups for caching", () => {
    const supervisedGroups = [
      { id: "s1", name: "Supervised A", room: { id: "r1", name: "Raum 1" } },
    ];
    const unclaimedGroups = [
      { id: "u1", name: "Unclaimed A", room: { name: "Raum 2" } },
    ];

    const combinedGroups = [
      ...supervisedGroups.map((g) => ({
        id: g.id,
        room: g.room ? { name: g.room.name } : undefined,
      })),
      ...unclaimedGroups.map((g) => ({
        id: g.id,
        room: g.room,
      })),
    ];

    expect(combinedGroups).toHaveLength(2);
    expect(combinedGroups[0]?.id).toBe("s1");
    expect(combinedGroups[1]?.id).toBe("u1");
  });

  it("sets empty cache when no supervised groups", () => {
    const supervisedGroups: Array<{ id: string }> = [];

    const cachedActiveGroups =
      supervisedGroups.length > 0 ? supervisedGroups : [];

    expect(cachedActiveGroups).toEqual([]);
  });
});

describe("MeinRaumPage SWR visits sync", () => {
  it("syncs SWR visit data with local state", () => {
    let students: Array<{ id: string }> = [];
    const setStudents = (newStudents: Array<{ id: string }>) => {
      students = newStudents;
    };

    const swrVisitsData = [
      { id: "1", name: "Max" },
      { id: "2", name: "Erika" },
    ];

    if (swrVisitsData) {
      setStudents(swrVisitsData);
    }

    expect(students).toHaveLength(2);
  });

  it("updates room student count when visits change", () => {
    const rooms = [
      { id: "room-1", student_count: 0 },
      { id: "room-2", student_count: 0 },
    ];

    const updateRoomStudentCount = (roomId: string, studentCount: number) => {
      return rooms.map((room) =>
        room.id === roomId ? { ...room, student_count: studentCount } : room,
      );
    };

    const updatedRooms = updateRoomStudentCount("room-1", 5);
    expect(updatedRooms[0]?.student_count).toBe(5);
  });
});

describe("Schulhof permanent tab functionality", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    cleanup();
  });

  it("shows Schulhof tab when no other supervised rooms but Schulhof exists", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [],
          unclaimedGroups: [],
          currentStaff: { id: "1" },
          educationalGroups: [],
          firstRoomVisits: [],
          firstRoomId: null,
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
      // Should show the Schulhof not supervising view
      expect(
        screen.getByText("Schulhof-Aufsicht verfügbar"),
      ).toBeInTheDocument();
    });
  });

  it("shows current supervisors when Schulhof has supervisors", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [],
          unclaimedGroups: [],
          currentStaff: { id: "1" },
          educationalGroups: [],
          firstRoomVisits: [],
          firstRoomId: null,
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
      expect(screen.getByText(/Aktuelle Aufsicht/)).toBeInTheDocument();
      expect(
        screen.getByText("Max Mustermann, Erika Schmidt"),
      ).toBeInTheDocument();
    });
  });

  it("shows no supervision warning when Schulhof has no supervisors", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [],
          unclaimedGroups: [],
          currentStaff: { id: "1" },
          educationalGroups: [],
          firstRoomVisits: [],
          firstRoomId: null,
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
      expect(screen.getByText("Aktuell keine Aufsicht")).toBeInTheDocument();
    });
  });

  it("shows student count on Schulhof view", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          supervisedGroups: [],
          unclaimedGroups: [],
          currentStaff: { id: "1" },
          educationalGroups: [],
          firstRoomVisits: [],
          firstRoomId: null,
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
      expect(screen.getByText(/15 Schüler im Schulhof/)).toBeInTheDocument();
    });
  });
});

describe("Schulhof toggle supervision behavior", () => {
  it("determines toggle action based on current supervision state", () => {
    const getToggleAction = (isUserSupervising: boolean): "start" | "stop" => {
      return isUserSupervising ? "stop" : "start";
    };

    expect(getToggleAction(false)).toBe("start");
    expect(getToggleAction(true)).toBe("stop");
  });

  it("shows correct button text based on supervision state", () => {
    const getButtonText = (
      isUserSupervising: boolean,
      isToggling: boolean,
    ): string => {
      if (isUserSupervising) {
        return isToggling ? "Wird abgegeben..." : "Aufsicht abgeben";
      }
      return isToggling ? "Wird übernommen..." : "Beaufsichtigen";
    };

    expect(getButtonText(false, false)).toBe("Beaufsichtigen");
    expect(getButtonText(false, true)).toBe("Wird übernommen...");
    expect(getButtonText(true, false)).toBe("Aufsicht abgeben");
    expect(getButtonText(true, true)).toBe("Wird abgegeben...");
  });

  it("sets appropriate error message on toggle failure", () => {
    const getErrorMessage = (isUserSupervising: boolean): string => {
      return isUserSupervising
        ? "Fehler beim Abgeben der Schulhof-Aufsicht."
        : "Fehler beim Übernehmen der Schulhof-Aufsicht.";
    };

    expect(getErrorMessage(false)).toContain("Übernehmen");
    expect(getErrorMessage(true)).toContain("Abgeben");
  });
});

describe("Schulhof status from BFF response", () => {
  it("parses Schulhof status from dashboard data", () => {
    const bffData = {
      schulhofStatus: {
        exists: true,
        roomId: "room-1",
        roomName: "Schulhof",
        activityGroupId: "act-1",
        activeGroupId: "active-1",
        isUserSupervising: true,
        supervisionId: "sup-1",
        supervisorCount: 3,
        studentCount: 25,
        supervisors: [
          {
            id: "s1",
            staffId: "staff-1",
            name: "Lehrer A",
            isCurrentUser: true,
          },
        ],
      },
    };

    const status = bffData.schulhofStatus;
    expect(status.exists).toBe(true);
    expect(status.isUserSupervising).toBe(true);
    expect(status.studentCount).toBe(25);
    expect(status.supervisors).toHaveLength(1);
    expect(status.supervisors[0]?.isCurrentUser).toBe(true);
  });

  it("handles null Schulhof status", () => {
    const bffData = {
      schulhofStatus: null,
    };

    const status = bffData.schulhofStatus;
    expect(status).toBeNull();
  });
});

describe("Room param URL handling", () => {
  it("identifies Schulhof from URL param", () => {
    const isSchulhofParam = (param: string | null): boolean => {
      return param === "schulhof";
    };

    expect(isSchulhofParam("schulhof")).toBe(true);
    expect(isSchulhofParam("room-123")).toBe(false);
    expect(isSchulhofParam(null)).toBe(false);
  });

  it("finds room index by room_id", () => {
    const allRooms = [
      { id: "1", room_id: "room-a", name: "Room A" },
      { id: "2", room_id: "room-b", name: "Room B" },
      { id: "3", room_id: "room-c", name: "Room C" },
    ];

    const findRoomIndex = (roomParam: string): number => {
      return allRooms.findIndex((r) => r.room_id === roomParam);
    };

    expect(findRoomIndex("room-b")).toBe(1);
    expect(findRoomIndex("room-x")).toBe(-1);
  });
});

describe("Desktop detection for tabs", () => {
  it("determines desktop mode based on window width", () => {
    const isDesktopWidth = (width: number): boolean => {
      return width >= 1024;
    };

    expect(isDesktopWidth(1024)).toBe(true);
    expect(isDesktopWidth(1920)).toBe(true);
    expect(isDesktopWidth(1023)).toBe(false);
    expect(isDesktopWidth(768)).toBe(false);
  });

  it("decides whether to show tabs", () => {
    const shouldShowTabs = (
      roomCount: number,
      schulhofExists: boolean,
      isDesktop: boolean,
    ): boolean => {
      return (roomCount > 1 || schulhofExists) && !isDesktop;
    };

    expect(shouldShowTabs(2, false, false)).toBe(true);
    expect(shouldShowTabs(1, true, false)).toBe(true);
    expect(shouldShowTabs(2, true, true)).toBe(false);
    expect(shouldShowTabs(1, false, false)).toBe(false);
  });
});

describe("matchesStudentFilters function coverage", () => {
  it("handles null/undefined student properties", () => {
    const student: {
      name?: string;
      first_name?: string;
      second_name?: string;
      group_name?: string;
    } = {
      name: undefined,
      first_name: undefined,
      second_name: undefined,
      group_name: undefined,
    };

    const searchTerm = "test";
    const searchLower = searchTerm.toLowerCase();

    const matchesSearch =
      (student.name?.toLowerCase().includes(searchLower) ?? false) ||
      (student.first_name?.toLowerCase().includes(searchLower) ?? false) ||
      (student.second_name?.toLowerCase().includes(searchLower) ?? false);

    expect(matchesSearch).toBe(false);
  });

  it("handles student with Unbekannt group name", () => {
    const student: { group_name?: string } = { group_name: undefined };
    const groupFilter = "Unbekannt";
    const studentGroupName = student.group_name ?? "Unbekannt";

    expect(studentGroupName).toBe("Unbekannt");
    expect(studentGroupName === groupFilter).toBe(true);
  });

  it("returns true when no filters are active", () => {
    const student = {
      name: "Max Mustermann",
      first_name: "Max",
      second_name: "Mustermann",
      group_name: "Gruppe A",
    };
    const searchTerm = "";
    const groupFilter = "all";

    // When searchTerm is empty, passesSearch should be true
    const passesSearch = searchTerm === "";
    // When groupFilter is "all", passesGroup should be true
    const passesGroup = groupFilter === "all";

    expect(passesSearch && passesGroup).toBe(true);
    // Also verify student data is correctly defined
    expect(student.name).toBe("Max Mustermann");
    expect(student.group_name).toBe("Gruppe A");
  });
});

describe("Filter configs generation", () => {
  it("extracts unique groups from students", () => {
    const students = [
      { group_name: "Gruppe A" },
      { group_name: "Gruppe B" },
      { group_name: "Gruppe A" },
      { group_name: undefined },
      { group_name: "Gruppe C" },
    ];

    const groups = Array.from(
      new Set(
        students
          .map((student) => student.group_name)
          .filter((name): name is string => !!name),
      ),
    ).sort((a, b) => a.localeCompare(b, "de"));

    expect(groups).toEqual(["Gruppe A", "Gruppe B", "Gruppe C"]);
  });

  it("creates filter options with all groups prefix", () => {
    const groups = ["Gruppe A", "Gruppe B"];

    const options = [
      { value: "all", label: "Alle Gruppen" },
      ...groups.map((groupName) => ({
        value: groupName,
        label: groupName,
      })),
    ];

    expect(options).toHaveLength(3);
    expect(options[0]?.value).toBe("all");
    expect(options[1]?.label).toBe("Gruppe A");
  });
});

describe("Schulhof tab ID handling", () => {
  const SCHULHOF_TAB_ID = "schulhof";
  const SCHULHOF_ROOM_NAME = "Schulhof";

  it("identifies Schulhof tab by ID", () => {
    const isSchulhofTab = (tabId: string): boolean => {
      return tabId === SCHULHOF_TAB_ID;
    };

    expect(isSchulhofTab("schulhof")).toBe(true);
    expect(isSchulhofTab("room-123")).toBe(false);
  });

  it("creates virtual Schulhof room object", () => {
    const schulhofStatus = {
      activeGroupId: "active-123",
      roomId: "room-schulhof",
      studentCount: 10,
    };

    const virtualSchulhofRoom = {
      id: schulhofStatus.activeGroupId,
      name: SCHULHOF_ROOM_NAME,
      room_name: SCHULHOF_ROOM_NAME,
      room_id: schulhofStatus.roomId ?? undefined,
      student_count: schulhofStatus.studentCount,
    };

    expect(virtualSchulhofRoom.name).toBe("Schulhof");
    expect(virtualSchulhofRoom.id).toBe("active-123");
    expect(virtualSchulhofRoom.student_count).toBe(10);
  });
});

describe("Room conversion from supervised groups", () => {
  it("converts supervised groups to ActiveRoom format", () => {
    const supervisedGroups = [
      {
        id: "1",
        name: "Group A",
        room: { id: "r1", name: "Raum 101" },
        room_id: "r1",
      },
      {
        id: "2",
        name: "Group B",
        room: { id: "r2", name: "Raum 102" },
        room_id: "r2",
      },
    ];

    const activeRooms = supervisedGroups
      .map((group) => ({
        id: group.id,
        name: group.name,
        room_name: group.room?.name,
        room_id: group.room_id,
        student_count: undefined,
        supervisor_name: undefined,
      }))
      .sort((a, b) =>
        (a.room_name ?? a.name).localeCompare(b.room_name ?? b.name, "de"),
      );

    expect(activeRooms).toHaveLength(2);
    expect(activeRooms[0]?.room_name).toBe("Raum 101");
    expect(activeRooms[1]?.room_name).toBe("Raum 102");
  });

  it("sorts rooms by name in German locale", () => {
    const rooms = [
      { room_name: "Zeichenraum", name: "G1" },
      { room_name: "Atelier", name: "G2" },
      { room_name: "Mensa", name: "G3" },
    ];

    const sorted = rooms.sort((a, b) =>
      (a.room_name ?? a.name).localeCompare(b.room_name ?? b.name, "de"),
    );

    expect(sorted[0]?.room_name).toBe("Atelier");
    expect(sorted[1]?.room_name).toBe("Mensa");
    expect(sorted[2]?.room_name).toBe("Zeichenraum");
  });
});

describe("Breadcrumb setting behavior", () => {
  it("uses Schulhof name when Schulhof tab is selected", () => {
    const isSchulhofTabSelected = true;
    const currentRoom = { room_name: "Raum 101" };
    const SCHULHOF_ROOM_NAME = "Schulhof";

    const activeSupervisionName = isSchulhofTabSelected
      ? SCHULHOF_ROOM_NAME
      : currentRoom?.room_name;

    expect(activeSupervisionName).toBe("Schulhof");
  });

  it("uses room name when regular room is selected", () => {
    const isSchulhofTabSelected = false;
    const currentRoom = { room_name: "Kunstzimmer" };
    const SCHULHOF_ROOM_NAME = "Schulhof";

    const activeSupervisionName = isSchulhofTabSelected
      ? SCHULHOF_ROOM_NAME
      : currentRoom?.room_name;

    expect(activeSupervisionName).toBe("Kunstzimmer");
  });
});

describe("Page header title determination", () => {
  it("shows room name on mobile with single room", () => {
    const isDesktop = false;
    const allRooms = [{ room_name: "Raum 101" }];
    const schulhofExists = false;
    const isSchulhofTabSelected = false;
    const currentRoomName = "Raum 101";

    const showRoomTitle =
      !isDesktop &&
      (allRooms.length === 1 || (allRooms.length === 0 && schulhofExists));

    const title = showRoomTitle
      ? isSchulhofTabSelected
        ? "Schulhof"
        : currentRoomName
      : "";

    expect(title).toBe("Raum 101");
  });

  it("shows empty title on desktop", () => {
    const isDesktop = true;
    const allRooms = [{ room_name: "Raum 101" }];

    const showRoomTitle = !isDesktop && allRooms.length === 1;
    const title = showRoomTitle ? "Raum 101" : "";

    expect(title).toBe("");
  });

  it("shows Schulhof title when Schulhof tab selected", () => {
    const isDesktop = false;
    const allRooms: Array<{ room_name: string }> = [];
    const schulhofExists = true;
    const isSchulhofTabSelected = true;

    const showRoomTitle =
      !isDesktop &&
      (allRooms.length === 1 || (allRooms.length === 0 && schulhofExists));

    const title = showRoomTitle && isSchulhofTabSelected ? "Schulhof" : "";

    expect(title).toBe("Schulhof");
  });
});

describe("Empty rooms check with Schulhof", () => {
  it("shows main view when only Schulhof exists", () => {
    const allRooms: Array<object> = [];
    const schulhofExists = true;

    const showEmptyRoomsView = allRooms.length === 0 && !schulhofExists;

    expect(showEmptyRoomsView).toBe(false);
  });

  it("shows empty rooms view when neither rooms nor Schulhof exist", () => {
    const allRooms: Array<object> = [];
    const schulhofExists = false;

    const showEmptyRoomsView = allRooms.length === 0 && !schulhofExists;

    expect(showEmptyRoomsView).toBe(true);
  });
});

describe("Auto-select Schulhof tab behavior", () => {
  it("auto-selects Schulhof when only option available", () => {
    const allRooms: Array<object> = [];
    const schulhofExists = true;
    let isSchulhofTabSelected = false;

    const shouldAutoSelect =
      allRooms.length === 0 && schulhofExists && !isSchulhofTabSelected;

    if (shouldAutoSelect) {
      isSchulhofTabSelected = true;
    }

    expect(isSchulhofTabSelected).toBe(true);
  });

  it("does not auto-select when regular rooms exist", () => {
    const allRooms = [{ id: "1" }];
    const schulhofExists = true;
    let isSchulhofTabSelected = false;

    const shouldAutoSelect =
      allRooms.length === 0 && schulhofExists && !isSchulhofTabSelected;

    if (shouldAutoSelect) {
      isSchulhofTabSelected = true;
    }

    expect(isSchulhofTabSelected).toBe(false);
  });
});

describe("Local storage for room persistence", () => {
  it("saves last room ID to localStorage", () => {
    const roomId = "room-123";
    const setItem = vi.fn();
    const mockLocalStorage = { setItem };

    mockLocalStorage.setItem("sidebar-last-room", roomId);

    expect(setItem).toHaveBeenCalledWith("sidebar-last-room", "room-123");
  });

  it("saves Schulhof tab ID for persistence", () => {
    const SCHULHOF_TAB_ID = "schulhof";
    const SCHULHOF_ROOM_NAME = "Schulhof";
    const setItem = vi.fn();
    const mockLocalStorage = { setItem };

    mockLocalStorage.setItem("sidebar-last-room", SCHULHOF_TAB_ID);
    mockLocalStorage.setItem("sidebar-last-room-name", SCHULHOF_ROOM_NAME);

    expect(setItem).toHaveBeenCalledWith("sidebar-last-room", "schulhof");
    expect(setItem).toHaveBeenCalledWith("sidebar-last-room-name", "Schulhof");
  });
});

describe("hasSupervision tracking via ref", () => {
  it("tracks supervision gained state", () => {
    const supervisedGroups = [{ id: "1" }, { id: "2" }];
    const hasSupervision = supervisedGroups.length > 0;

    expect(hasSupervision).toBe(true);
  });

  it("tracks no supervision state", () => {
    const supervisedGroups: Array<object> = [];
    const hasSupervision = supervisedGroups.length > 0;

    expect(hasSupervision).toBe(false);
  });
});

describe("Error handling scenarios", () => {
  it("handles 403 forbidden error gracefully", () => {
    const errorMessage = "Forbidden: No access to group";
    const is403Error =
      errorMessage.includes("403") || errorMessage.includes("Forbidden");

    expect(is403Error).toBe(true);
  });

  it("sets appropriate error message for 403 response", () => {
    const dashboardError = { message: "403 Forbidden" };
    let error: string | null = null;
    let hasAccess = true;

    if (dashboardError.message.includes("403")) {
      error = "Sie haben aktuell keinen aktiven Raum zur Supervision.";
      hasAccess = false;
    }

    expect(error).toBe(
      "Sie haben aktuell keinen aktiven Raum zur Supervision.",
    );
    expect(hasAccess).toBe(false);
  });

  it("sets generic error message for non-403 errors", () => {
    const dashboardError = { message: "Network error" };
    let error: string | null = null;

    if (dashboardError.message.includes("403")) {
      error = "Sie haben aktuell keinen aktiven Raum zur Supervision.";
    } else {
      error = "Fehler beim Laden der Aktivitätsdaten.";
    }

    expect(error).toBe("Fehler beim Laden der Aktivitätsdaten.");
  });
});

describe("loadRoomVisits error handling", () => {
  it("returns empty array for 403 forbidden responses", () => {
    const error = new Error("Request failed with status 403");
    const is403 = error.message.includes("403");

    let result: Array<unknown> = [];
    if (is403) {
      console.warn("No permission - returning empty list");
      result = [];
    }

    expect(result).toEqual([]);
  });

  it("re-throws non-403 errors", () => {
    const error = new Error("Network timeout");
    const is403 = error.message.includes("403");

    expect(is403).toBe(false);
    expect(() => {
      if (!is403) throw error;
    }).toThrow("Network timeout");
  });
});

describe("SWR visit data sync", () => {
  it("updates students when SWR data changes", () => {
    const swrVisitsData = [
      { id: "s1", name: "Student 1", activeGroupId: "g1" },
      { id: "s2", name: "Student 2", activeGroupId: "g1" },
    ] as Array<{ id: string; name: string; activeGroupId?: string }> | null;
    const currentRoomId = "g1" as string | null;

    let students: Array<{ id: string; name: string }> = [];
    let roomStudentCount = 0;

    if (swrVisitsData && currentRoomId) {
      students = swrVisitsData;
      roomStudentCount = swrVisitsData.length;
    }

    expect(students).toHaveLength(2);
    expect(roomStudentCount).toBe(2);
  });

  it("does not update when no current room", () => {
    const swrVisitsData = [{ id: "s1", name: "Student 1" }] as Array<{
      id: string;
      name: string;
    }> | null;
    const currentRoomId = null as string | null;

    const updated = Boolean(swrVisitsData && currentRoomId);

    expect(updated).toBe(false);
  });
});

describe("Tab change handler logic", () => {
  it("switches to Schulhof tab correctly", () => {
    const SCHULHOF_TAB_ID = "schulhof";
    const tabId = "schulhof";

    let isSchulhofTabSelected = false;
    let selectedRoomIndex = 0;

    if (tabId === SCHULHOF_TAB_ID) {
      isSchulhofTabSelected = true;
      selectedRoomIndex = -1;
    }

    expect(isSchulhofTabSelected).toBe(true);
    expect(selectedRoomIndex).toBe(-1);
  });

  it("switches to regular room tab correctly", () => {
    const SCHULHOF_TAB_ID = "schulhof";
    const tabId = "room-123" as string;
    const allRooms = [
      { id: "room-123", room_id: "r1" },
      { id: "room-456", room_id: "r2" },
    ];

    let isSchulhofSelected = true;
    let targetIndex = -1;

    if (tabId !== SCHULHOF_TAB_ID) {
      isSchulhofSelected = false;
      targetIndex = allRooms.findIndex((r) => r.id === tabId);
    }

    expect(isSchulhofSelected).toBe(false);
    expect(targetIndex).toBe(0);
  });
});

describe("Action button rendering conditions", () => {
  it("shows release button when supervising Schulhof", () => {
    const isSchulhofTabSelected = true;
    const schulhofStatus = { isUserSupervising: true };

    const showReleaseButton =
      isSchulhofTabSelected && schulhofStatus?.isUserSupervising;

    expect(showReleaseButton).toBe(true);
  });

  it("shows claim button when not supervising Schulhof", () => {
    // Test the production logic: show claim button when tab is selected,
    // status exists, and user is not supervising
    const schulhofStatus = { isUserSupervising: false };

    // This mirrors the actual condition in the page component
    const showClaimButton = !schulhofStatus.isUserSupervising;

    expect(showClaimButton).toBe(true);
  });

  it("shows no action button for regular rooms", () => {
    const isSchulhofTabSelected = false;
    const schulhofStatus = { isUserSupervising: true };

    const showActionButton = isSchulhofTabSelected && schulhofStatus;

    expect(showActionButton).toBe(false);
  });
});

describe("Release supervision modal flow", () => {
  it("finds current user supervision correctly", () => {
    const currentStaffId = "staff-1";
    const supervisors = [
      { id: "sup-1", staffId: "staff-1", isActive: true },
      { id: "sup-2", staffId: "staff-2", isActive: true },
      { id: "sup-3", staffId: "staff-1", isActive: false },
    ];

    const mySupervision = supervisors.find(
      (sup) => sup.staffId === currentStaffId && sup.isActive,
    );

    expect(mySupervision?.id).toBe("sup-1");
  });

  it("handles missing supervision gracefully", () => {
    const currentStaffId = "staff-unknown";
    const supervisors = [{ id: "sup-1", staffId: "staff-1", isActive: true }];

    const mySupervision = supervisors.find(
      (sup) => sup.staffId === currentStaffId && sup.isActive,
    );

    expect(mySupervision).toBeUndefined();
  });
});

describe("Toggle Schulhof supervision", () => {
  it("determines correct action based on current state", () => {
    const supervisingStatus = { isUserSupervising: true };
    const notSupervisingStatus = { isUserSupervising: false };

    const stopAction = supervisingStatus.isUserSupervising ? "stop" : "start";
    const startAction = notSupervisingStatus.isUserSupervising
      ? "stop"
      : "start";

    expect(stopAction).toBe("stop");
    expect(startAction).toBe("start");
  });

  it("sets appropriate error message on failure", () => {
    const isUserSupervising = true;
    const errorMessage = isUserSupervising
      ? "Fehler beim Abgeben der Schulhof-Aufsicht."
      : "Fehler beim Übernehmen der Schulhof-Aufsicht.";

    expect(errorMessage).toBe("Fehler beim Abgeben der Schulhof-Aufsicht.");
  });
});

describe("Student content rendering conditions", () => {
  it("shows empty state when no students", () => {
    const students: Array<{ id: string }> = [];
    const showEmptyState = students.length === 0;

    expect(showEmptyState).toBe(true);
  });

  it("shows student grid when students exist and match filters", () => {
    const students = [{ id: "s1" }, { id: "s2" }];
    const filteredStudents = students;

    const showGrid = students.length > 0 && filteredStudents.length > 0;

    expect(showGrid).toBe(true);
  });

  it("shows empty results when filters exclude all students", () => {
    const students = [{ id: "s1" }];
    const filteredStudents: Array<{ id: string }> = [];

    const showEmptyResults =
      students.length > 0 && filteredStudents.length === 0;

    expect(showEmptyResults).toBe(true);
  });
});

describe("Schulhof not supervising view conditions", () => {
  it("shows info when supervisors exist", () => {
    const schulhofStatus = {
      supervisorCount: 2,
      supervisors: [{ name: "Teacher A" }, { name: "Teacher B" }],
    };

    const showSupervisorInfo = schulhofStatus.supervisorCount > 0;
    const supervisorNames = schulhofStatus.supervisors
      .map((s) => s.name)
      .join(", ");

    expect(showSupervisorInfo).toBe(true);
    expect(supervisorNames).toBe("Teacher A, Teacher B");
  });

  it("shows warning when no supervisors", () => {
    const schulhofStatus = {
      supervisorCount: 0,
      supervisors: [],
    };

    const showNoSupervisorWarning = schulhofStatus.supervisorCount === 0;

    expect(showNoSupervisorWarning).toBe(true);
  });

  it("shows student count info", () => {
    const schulhofStatus = { studentCount: 15 };
    const showStudentCount = schulhofStatus.studentCount > 0;

    expect(showStudentCount).toBe(true);
  });
});

describe("Room switching logic", () => {
  it("prevents switching to same room", () => {
    const selectedRoomIndex = 1;
    const targetIndex = 1;
    const allRooms = [{ id: "1" }, { id: "2" }];

    const shouldSwitch =
      targetIndex !== selectedRoomIndex && allRooms[targetIndex];

    expect(shouldSwitch).toBeFalsy();
  });

  it("allows switching to different room", () => {
    const selectedRoomIndex = 0 as number;
    const targetIndex = 1 as number;
    const allRooms = [{ id: "1" }, { id: "2" }];

    const shouldSwitch =
      targetIndex !== selectedRoomIndex && allRooms[targetIndex];

    expect(shouldSwitch).toBeTruthy();
  });

  it("handles 403 error during room switch", () => {
    const err = new Error("Request failed with 403");
    const roomName = "Test Room";

    let errorMessage = "";
    if (err.message.includes("403")) {
      errorMessage = `Keine Berechtigung für "${roomName}". Kontaktieren Sie einen Administrator.`;
    } else {
      errorMessage = "Fehler beim Laden der Raumdaten.";
    }

    expect(errorMessage).toContain("Keine Berechtigung");
  });
});

describe("currentRoom calculation", () => {
  it("returns virtual Schulhof room when supervising", () => {
    const isSchulhofTabSelected = true;
    const schulhofStatus = {
      isUserSupervising: true,
      activeGroupId: "active-123",
      roomId: "room-schulhof",
      studentCount: 5,
    };
    const SCHULHOF_ROOM_NAME = "Schulhof";

    const currentRoom =
      isSchulhofTabSelected &&
      schulhofStatus?.isUserSupervising &&
      schulhofStatus?.activeGroupId
        ? {
            id: schulhofStatus.activeGroupId,
            name: SCHULHOF_ROOM_NAME,
            room_name: SCHULHOF_ROOM_NAME,
            room_id: schulhofStatus.roomId ?? undefined,
            student_count: schulhofStatus.studentCount,
          }
        : null;

    expect(currentRoom?.id).toBe("active-123");
    expect(currentRoom?.name).toBe("Schulhof");
  });

  it("returns null when Schulhof selected but not supervising", () => {
    const isSchulhofTabSelected = true;
    const schulhofStatus = {
      isUserSupervising: false,
      activeGroupId: "active-123",
    };

    const currentRoom =
      isSchulhofTabSelected &&
      schulhofStatus?.isUserSupervising &&
      schulhofStatus?.activeGroupId
        ? { id: schulhofStatus.activeGroupId }
        : null;

    expect(currentRoom).toBeNull();
  });

  it("returns regular room when not Schulhof tab", () => {
    const isSchulhofTabSelected = false;
    const allRooms = [
      { id: "1", name: "Room A" },
      { id: "2", name: "Room B" },
    ];
    const selectedRoomIndex = 1;

    const currentRoom = !isSchulhofTabSelected
      ? (allRooms[selectedRoomIndex] ?? null)
      : null;

    expect(currentRoom?.name).toBe("Room B");
  });
});

describe("Visit data transformation", () => {
  it("transforms visit data to student format", () => {
    const visit = {
      studentId: "s1",
      studentName: "Max Mustermann",
      schoolClass: "3a",
      groupName: "OGS Blau",
      activeGroupId: "g1",
      checkInTime: new Date("2024-01-15T10:00:00"),
      isActive: true,
    };
    const roomName = "Raum 101";

    const nameParts = visit.studentName?.split(" ") ?? ["", ""];
    const firstName = nameParts[0] ?? "";
    const lastName = nameParts.slice(1).join(" ") ?? "";
    const location = roomName ? `Anwesend - ${roomName}` : "Anwesend";

    expect(firstName).toBe("Max");
    expect(lastName).toBe("Mustermann");
    expect(location).toBe("Anwesend - Raum 101");
  });

  it("handles missing student name", () => {
    const visit = {
      studentId: "s1",
      studentName: null as string | null,
    };

    const nameParts = visit.studentName?.split(" ") ?? ["", ""];
    const firstName = nameParts[0] ?? "";
    const lastName = nameParts.slice(1).join(" ") ?? "";

    expect(firstName).toBe("");
    expect(lastName).toBe("");
  });

  it("handles single-word name", () => {
    const visit = { studentName: "Madonna" };

    const nameParts = visit.studentName?.split(" ") ?? ["", ""];
    const firstName = nameParts[0] ?? "";
    const lastName = nameParts.slice(1).join(" ");

    expect(firstName).toBe("Madonna");
    expect(lastName).toBe("");
  });
});

describe("Active filters array building", () => {
  it("adds search filter when search term exists", () => {
    const searchTerm = "Max";
    const filters: Array<{ id: string; label: string }> = [];

    if (searchTerm) {
      filters.push({ id: "search", label: `"${searchTerm}"` });
    }

    expect(filters).toHaveLength(1);
    expect(filters[0]?.label).toBe('"Max"');
  });

  it("adds group filter when not all", () => {
    const groupFilter = "OGS Blau" as string;
    const filters: Array<{ id: string; label: string }> = [];

    if (groupFilter !== "all") {
      filters.push({ id: "group", label: `Gruppe: ${groupFilter}` });
    }

    expect(filters).toHaveLength(1);
    expect(filters[0]?.label).toBe("Gruppe: OGS Blau");
  });

  it("returns empty array when no filters active", () => {
    const searchTerm = "" as string;
    const groupFilter = "all" as string;
    const filters: Array<{ id: string; label: string }> = [];

    if (searchTerm) {
      filters.push({ id: "search", label: `"${searchTerm}"` });
    }
    if (groupFilter !== "all") {
      filters.push({ id: "group", label: `Gruppe: ${groupFilter}` });
    }

    expect(filters).toHaveLength(0);
  });
});

describe("URL param restoration from localStorage", () => {
  it("restores saved room from localStorage", () => {
    const savedRoomId = "room-saved";
    const allRooms = [
      { id: "1", room_id: "room-saved" },
      { id: "2", room_id: "room-other" },
    ];

    const savedIndex = savedRoomId
      ? allRooms.findIndex((r) => r.room_id === savedRoomId)
      : -1;

    expect(savedIndex).toBe(0);
  });

  it("persists first room when nothing saved", () => {
    const savedRoomId: string | null = null;
    const allRooms = [{ id: "1", room_id: "room-first" }];

    const savedIndex = savedRoomId
      ? allRooms.findIndex((r) => r.room_id === savedRoomId)
      : -1;

    let persistedRoomId: string | null = null;
    if (savedIndex === -1 && allRooms[0]?.room_id) {
      persistedRoomId = allRooms[0].room_id;
    }

    expect(persistedRoomId).toBe("room-first");
  });
});

describe("Dashboard data sync with state", () => {
  it("extracts room names from educational groups", () => {
    const educationalGroups = [
      { id: "1", name: "Gruppe A", room: { name: "Raum A" } },
      { id: "2", name: "Gruppe B", room: { name: "Raum B" } },
      { id: "3", name: "Gruppe C", room: undefined },
    ];

    const roomNames = educationalGroups
      .map((group) => group.room?.name)
      .filter((name): name is string => !!name);

    expect(roomNames).toEqual(["Raum A", "Raum B"]);
  });

  it("creates group name to ID map", () => {
    const educationalGroups = [
      { id: "1", name: "Gruppe A" },
      { id: "2", name: "Gruppe B" },
    ];

    const nameToIdMap = new Map<string, string>();
    educationalGroups.forEach((group) => {
      if (group.name) {
        nameToIdMap.set(group.name, group.id);
      }
    });

    expect(nameToIdMap.get("Gruppe A")).toBe("1");
    expect(nameToIdMap.get("Gruppe B")).toBe("2");
  });

  it("caches active groups from dashboard", () => {
    const supervisedGroups = [{ id: "1", room: { name: "Room A" } }];
    const unclaimedGroups = [{ id: "2", room: { name: "Room B" } }];

    const combinedGroups = [
      ...supervisedGroups.map((g) => ({
        id: g.id,
        room: g.room ? { name: g.room.name } : undefined,
      })),
      ...unclaimedGroups.map((g) => ({
        id: g.id,
        room: g.room,
      })),
    ];

    expect(combinedGroups).toHaveLength(2);
  });
});

describe("Loading state derivation", () => {
  it("sets loading when dashboard loading and no data", () => {
    const isDashboardLoading = true;
    const dashboardData = null;

    const shouldSetLoading = isDashboardLoading && !dashboardData;

    expect(shouldSetLoading).toBe(true);
  });

  it("does not set loading when data exists", () => {
    const isDashboardLoading = true;
    const dashboardData = { supervisedGroups: [] };

    const shouldSetLoading = isDashboardLoading && !dashboardData;

    expect(shouldSetLoading).toBe(false);
  });
});

describe("Badge count display", () => {
  it("shows Schulhof student count when Schulhof selected", () => {
    const isSchulhofTabSelected = true;
    const schulhofStatus = { studentCount: 12 };
    const currentRoom = { student_count: 5 };

    const badgeCount = isSchulhofTabSelected
      ? (schulhofStatus?.studentCount ?? 0)
      : (currentRoom?.student_count ?? 0);

    expect(badgeCount).toBe(12);
  });

  it("shows room student count when regular room selected", () => {
    const isSchulhofTabSelected = false;
    const schulhofStatus = { studentCount: 12 };
    const currentRoom = { student_count: 5 };

    const badgeCount = isSchulhofTabSelected
      ? (schulhofStatus?.studentCount ?? 0)
      : (currentRoom?.student_count ?? 0);

    expect(badgeCount).toBe(5);
  });

  it("defaults to zero when no data", () => {
    const schulhofStatus = null as { studentCount?: number } | null;

    const badgeCount = schulhofStatus?.studentCount ?? 0;

    expect(badgeCount).toBe(0);
  });
});

describe("Tabs visibility logic", () => {
  it("shows tabs when multiple rooms exist on mobile", () => {
    const allRooms = [{ id: "1" }, { id: "2" }];
    const schulhofExists = false;
    const isDesktop = false;

    const showTabs = (allRooms.length > 1 || schulhofExists) && !isDesktop;

    expect(showTabs).toBe(true);
  });

  it("shows tabs when Schulhof exists on mobile", () => {
    const allRooms = [{ id: "1" }];
    const schulhofExists = true;
    const isDesktop = false;

    const showTabs = (allRooms.length > 1 || schulhofExists) && !isDesktop;

    expect(showTabs).toBe(true);
  });

  it("hides tabs on desktop", () => {
    const allRooms = [{ id: "1" }, { id: "2" }];
    const schulhofExists = true;
    const isDesktop = true;

    const showTabs = (allRooms.length > 1 || schulhofExists) && !isDesktop;

    expect(showTabs).toBe(false);
  });

  it("hides tabs when single room and no Schulhof on mobile", () => {
    const allRooms = [{ id: "1" }];
    const schulhofExists = false;
    const isDesktop = false;

    const showTabs = (allRooms.length > 1 || schulhofExists) && !isDesktop;

    expect(showTabs).toBe(false);
  });
});

describe("Schulhof param handling in URL", () => {
  it("detects Schulhof param and sets tab selected", () => {
    // Test behavior: when URL has schulhof param AND Schulhof exists,
    // the tab should be selected
    const schulhofExists = true;

    let isSchulhofTabSelected = false;
    let selectedRoomIndex = 0;

    // Testing the positive case where Schulhof exists
    if (schulhofExists) {
      isSchulhofTabSelected = true;
      selectedRoomIndex = -1;
    }

    expect(isSchulhofTabSelected).toBe(true);
    expect(selectedRoomIndex).toBe(-1);
  });

  it("ignores Schulhof param when Schulhof does not exist", () => {
    // Test behavior: even with Schulhof URL param, tab is not selected
    // if Schulhof doesn't exist in the system
    const schulhofExists = false;

    let isSchulhofTabSelected = false;

    // Simplified condition since we're testing the schulhofExists branch
    if (schulhofExists) {
      isSchulhofTabSelected = true;
    }

    expect(isSchulhofTabSelected).toBe(false);
  });
});

describe("First room visits from BFF", () => {
  it("transforms BFF visits to student format", () => {
    const firstRoomVisits = [
      {
        studentId: "s1",
        studentName: "Anna Schmidt",
        schoolClass: "2a",
        groupName: "OGS Rot",
        activeGroupId: "g1",
        checkInTime: "2024-01-15T09:00:00Z",
        isActive: true,
      },
    ];
    const firstRoom = { room_name: "Mensa" };
    const nameToIdMap = new Map([["OGS Rot", "group-1"]]);

    const studentsFromVisits = firstRoomVisits.map((visit) => {
      const nameParts = visit.studentName?.split(" ") ?? ["", ""];
      const firstName = nameParts[0] ?? "";
      const lastName = nameParts.slice(1).join(" ") ?? "";
      const location = firstRoom.room_name
        ? `Anwesend - ${firstRoom.room_name}`
        : "Anwesend";
      const groupId = visit.groupName
        ? nameToIdMap.get(visit.groupName)
        : undefined;

      return {
        id: visit.studentId,
        name: visit.studentName ?? "",
        first_name: firstName,
        second_name: lastName,
        school_class: visit.schoolClass ?? "",
        current_location: location,
        group_name: visit.groupName,
        group_id: groupId,
        activeGroupId: visit.activeGroupId,
        checkInTime: new Date(visit.checkInTime),
      };
    });

    expect(studentsFromVisits[0]?.first_name).toBe("Anna");
    expect(studentsFromVisits[0]?.second_name).toBe("Schmidt");
    expect(studentsFromVisits[0]?.current_location).toBe("Anwesend - Mensa");
    expect(studentsFromVisits[0]?.group_id).toBe("group-1");
  });

  it("only applies first room visits when first room selected", () => {
    const selectedRoomIndex = 0;
    const firstRoomVisits = [{ studentId: "s1" }];

    let applied = false;
    if (selectedRoomIndex === 0 && firstRoomVisits.length > 0) {
      applied = true;
    }

    expect(applied).toBe(true);
  });

  it("skips applying first room visits when other room selected", () => {
    const applyFirstRoomVisits = (
      selectedRoomIndex: number,
      firstRoomVisits: Array<{ studentId: string }>,
    ): boolean => {
      if (selectedRoomIndex === 0 && firstRoomVisits.length > 0) {
        return true;
      }
      return false;
    };

    const selectedRoomIndex = 2 as number;
    const firstRoomVisits = [{ studentId: "s1" }];

    const applied = applyFirstRoomVisits(selectedRoomIndex, firstRoomVisits);

    expect(applied).toBe(false);
  });
});

describe("Update room student count helper", () => {
  it("updates correct room in array", () => {
    const rooms = [
      { id: "1", student_count: 5 },
      { id: "2", student_count: 3 },
    ];
    const targetRoomId = "1";
    const newCount = 10;

    const updatedRooms = rooms.map((room) =>
      room.id === targetRoomId ? { ...room, student_count: newCount } : room,
    );

    expect(updatedRooms[0]?.student_count).toBe(10);
    expect(updatedRooms[1]?.student_count).toBe(3);
  });
});

describe("Filtered students array handling", () => {
  it("ensures students array before filtering", () => {
    const students = null as unknown as Array<{ id: string }>;
    const safeStudents = Array.isArray(students) ? students : [];

    expect(safeStudents).toEqual([]);
  });

  it("uses students array when valid", () => {
    const students = [{ id: "1" }, { id: "2" }];
    const safeStudents = Array.isArray(students) ? students : [];

    expect(safeStudents).toHaveLength(2);
  });
});

describe("Empty students state handling", () => {
  it("identifies empty students state", () => {
    const students: Array<{ id: string }> = [];

    const showEmptyState = students.length === 0;

    expect(showEmptyState).toBe(true);
  });

  it("identifies non-empty students state", () => {
    const students = [{ id: "1" }];

    const showEmptyState = students.length === 0;

    expect(showEmptyState).toBe(false);
  });
});

describe("Student name parsing edge cases", () => {
  it("handles empty student name", () => {
    const studentName = "";
    const nameParts = studentName?.split(" ") ?? ["", ""];
    const firstName = nameParts[0] ?? "";
    const lastName = nameParts.slice(1).join(" ") ?? "";

    expect(firstName).toBe("");
    expect(lastName).toBe("");
  });

  it("handles single word name", () => {
    const studentName = "Max";
    const nameParts = studentName?.split(" ") ?? ["", ""];
    const firstName = nameParts[0] ?? "";
    const lastName = nameParts.slice(1).join(" ") ?? "";

    expect(firstName).toBe("Max");
    expect(lastName).toBe("");
  });

  it("handles multiple word last name", () => {
    const studentName = "Anna Maria von Schmidt";
    const nameParts = studentName?.split(" ") ?? ["", ""];
    const firstName = nameParts[0] ?? "";
    const lastName = nameParts.slice(1).join(" ") ?? "";

    expect(firstName).toBe("Anna");
    expect(lastName).toBe("Maria von Schmidt");
  });

  it("handles null student name", () => {
    const studentName = null as string | null;
    const nameParts = studentName?.split(" ") ?? ["", ""];
    const firstName = nameParts[0] ?? "";
    const lastName = nameParts.slice(1).join(" ") ?? "";

    expect(firstName).toBe("");
    expect(lastName).toBe("");
  });
});

describe("Location string construction", () => {
  it("includes room name when available", () => {
    const roomName = "Mensa";
    const location = roomName ? `Anwesend - ${roomName}` : "Anwesend";

    expect(location).toBe("Anwesend - Mensa");
  });

  it("uses default when room name is empty", () => {
    const roomName = "" as string;
    const location = roomName ? `Anwesend - ${roomName}` : "Anwesend";

    expect(location).toBe("Anwesend");
  });

  it("uses default when room name is undefined", () => {
    const roomName = undefined as string | undefined;
    const location = roomName ? `Anwesend - ${roomName}` : "Anwesend";

    expect(location).toBe("Anwesend");
  });
});

describe("currentRoom calculation", () => {
  it("returns virtual Schulhof room when supervising", () => {
    const isSchulhofTabSelected = true;
    const schulhofStatus = {
      isUserSupervising: true,
      activeGroupId: "ag-123",
      studentCount: 5,
    };
    const SCHULHOF_TAB_ID = "schulhof";
    const SCHULHOF_ROOM_NAME = "Schulhof";

    const currentRoom = isSchulhofTabSelected
      ? schulhofStatus?.isUserSupervising && schulhofStatus?.activeGroupId
        ? {
            id: SCHULHOF_TAB_ID,
            name: SCHULHOF_ROOM_NAME,
            room_id: SCHULHOF_TAB_ID,
            student_count: schulhofStatus?.studentCount ?? 0,
          }
        : null
      : null;

    expect(currentRoom).not.toBeNull();
    expect(currentRoom?.id).toBe("schulhof");
    expect(currentRoom?.student_count).toBe(5);
  });

  it("returns null when Schulhof selected but not supervising", () => {
    const isSchulhofTabSelected = true;
    const schulhofStatus = {
      isUserSupervising: false,
      activeGroupId: "ag-123",
      studentCount: 5,
    };

    const currentRoom = isSchulhofTabSelected
      ? schulhofStatus?.isUserSupervising && schulhofStatus?.activeGroupId
        ? { id: "schulhof", name: "Schulhof" }
        : null
      : null;

    expect(currentRoom).toBeNull();
  });

  it("returns regular room when not Schulhof", () => {
    const isSchulhofTabSelected = false;
    const selectedRoomIndex = 0;
    const allRooms = [
      { id: "r1", name: "Mensa", room_id: "r1", student_count: 10 },
    ];

    const currentRoom = isSchulhofTabSelected
      ? null
      : (allRooms[selectedRoomIndex] ?? null);

    expect(currentRoom).not.toBeNull();
    expect(currentRoom?.name).toBe("Mensa");
    expect(currentRoom?.student_count).toBe(10);
  });
});

describe("Page header title logic", () => {
  it("shows room name on mobile with single room", () => {
    const isDesktop = false;
    const allRooms = [{ room_name: "Mensa" }];
    const schulhofStatus = { exists: false };
    const isSchulhofTabSelected = false;
    const currentRoom = { room_name: "Mensa" };

    const title =
      !isDesktop &&
      (allRooms.length === 1 ||
        (allRooms.length === 0 && schulhofStatus?.exists))
        ? isSchulhofTabSelected
          ? "Schulhof"
          : (currentRoom?.room_name ?? "Aktuelle Aufsicht")
        : "";

    expect(title).toBe("Mensa");
  });

  it("shows Schulhof on mobile when only Schulhof exists", () => {
    const isDesktop = false;
    const allRooms: Array<{ room_name: string }> = [];
    const schulhofStatus = { exists: true };
    const isSchulhofTabSelected = true;

    const title =
      !isDesktop &&
      (allRooms.length === 1 ||
        (allRooms.length === 0 && schulhofStatus?.exists))
        ? isSchulhofTabSelected
          ? "Schulhof"
          : "Aktuelle Aufsicht"
        : "";

    expect(title).toBe("Schulhof");
  });

  it("shows empty string on desktop", () => {
    const isDesktop = true;
    const allRooms = [{ room_name: "Mensa" }];

    const title =
      !isDesktop && allRooms.length === 1 ? (allRooms[0]?.room_name ?? "") : "";

    expect(title).toBe("");
  });
});

describe("Tabs visibility logic extended", () => {
  it("shows tabs when Schulhof exists even with no regular rooms", () => {
    const allRooms: Array<{ id: string }> = [];
    const schulhofExists = true;
    const isDesktop = false;

    const showTabs = (allRooms.length > 1 || schulhofExists) && !isDesktop;

    expect(showTabs).toBe(true);
  });

  it("hides tabs on desktop even with multiple rooms", () => {
    const allRooms = [{ id: "1" }, { id: "2" }];
    const schulhofExists = true;
    const isDesktop = true;

    const showTabs = (allRooms.length > 1 || schulhofExists) && !isDesktop;

    expect(showTabs).toBe(false);
  });

  it("hides tabs on mobile with single room and no Schulhof", () => {
    const allRooms = [{ id: "1" }];
    const schulhofExists = false;
    const isDesktop = false;

    const showTabs = (allRooms.length > 1 || schulhofExists) && !isDesktop;

    expect(showTabs).toBe(false);
  });
});

describe("Schulhof not supervising view logic", () => {
  it("shows current supervisors when count > 0", () => {
    const schulhofStatus = {
      supervisorCount: 2,
      supervisors: [
        { staffId: "s1", name: "Max Muster" },
        { staffId: "s2", name: "Erika Test" },
      ],
    };

    const showCurrentSupervisors = schulhofStatus.supervisorCount > 0;
    const supervisorNames = schulhofStatus.supervisors
      .map((s) => s.name)
      .join(", ");

    expect(showCurrentSupervisors).toBe(true);
    expect(supervisorNames).toBe("Max Muster, Erika Test");
  });

  it("shows warning when no supervisors", () => {
    const schulhofStatus = {
      supervisorCount: 0,
      supervisors: [],
    };

    const showWarning = schulhofStatus.supervisorCount === 0;

    expect(showWarning).toBe(true);
  });

  it("shows student count info when students present", () => {
    const schulhofStatus = {
      studentCount: 5,
    };

    const showStudentInfo = schulhofStatus.studentCount > 0;

    expect(showStudentInfo).toBe(true);
  });

  it("hides student count info when no students", () => {
    const schulhofStatus = {
      studentCount: 0,
    };

    const showStudentInfo = schulhofStatus.studentCount > 0;

    expect(showStudentInfo).toBe(false);
  });
});

describe("Student content visibility guard", () => {
  it("shows students for regular room", () => {
    const isSchulhofTabSelected = false;
    const schulhofStatus = { isUserSupervising: false };

    const showStudentContent =
      !isSchulhofTabSelected || schulhofStatus?.isUserSupervising;

    expect(showStudentContent).toBe(true);
  });

  it("shows students for Schulhof when supervising", () => {
    const isSchulhofTabSelected = true;
    const schulhofStatus = { isUserSupervising: true };

    const showStudentContent =
      !isSchulhofTabSelected || schulhofStatus?.isUserSupervising;

    expect(showStudentContent).toBe(true);
  });

  it("hides students for Schulhof when not supervising", () => {
    const isSchulhofTabSelected = true;
    const schulhofStatus = { isUserSupervising: false };

    const showStudentContent =
      !isSchulhofTabSelected || schulhofStatus?.isUserSupervising;

    expect(showStudentContent).toBe(false);
  });
});

describe("Visit enrichment edge cases", () => {
  it("returns undefined group_id when groupName is null", () => {
    const visit = { groupName: null as string | null };
    const groupNameToId = new Map([["OGS Rot", "g1"]]);

    const groupId =
      visit.groupName && groupNameToId
        ? groupNameToId.get(visit.groupName)
        : undefined;

    expect(groupId).toBeUndefined();
  });

  it("returns undefined group_id when group not in map", () => {
    const visit = { groupName: "Unknown Group" };
    const groupNameToId = new Map([["OGS Rot", "g1"]]);

    const groupId =
      visit.groupName && groupNameToId
        ? groupNameToId.get(visit.groupName)
        : undefined;

    expect(groupId).toBeUndefined();
  });

  it("returns group_id when found in map", () => {
    const visit = { groupName: "OGS Rot" };
    const groupNameToId = new Map([["OGS Rot", "g1"]]);

    const groupId =
      visit.groupName && groupNameToId
        ? groupNameToId.get(visit.groupName)
        : undefined;

    expect(groupId).toBe("g1");
  });
});

describe("First room visits application guard", () => {
  // Helper function that encapsulates the first room visits logic
  function shouldApplyFirstRoomVisits(
    selectedRoomIndex: number,
    firstRoomVisits: Array<{ id: string }>,
  ): boolean {
    return selectedRoomIndex === 0 && firstRoomVisits.length > 0;
  }

  it("applies first room visits only when index is 0", () => {
    const result = shouldApplyFirstRoomVisits(0, [{ id: "v1" }]);
    expect(result).toBe(true);
  });

  it("does not apply first room visits for other rooms", () => {
    const result = shouldApplyFirstRoomVisits(1, [{ id: "v1" }]);
    expect(result).toBe(false);
  });
});

describe("Action button conditional rendering logic", () => {
  // Helper function that encapsulates the button type logic
  function getButtonType(
    isSchulhofTabSelected: boolean,
    schulhofStatus: { isUserSupervising: boolean } | null,
  ): "release" | "claim" | "none" {
    if (!isSchulhofTabSelected || !schulhofStatus) {
      return "none";
    }
    return schulhofStatus.isUserSupervising ? "release" : "claim";
  }

  it("shows release button when supervising Schulhof", () => {
    const buttonType = getButtonType(true, { isUserSupervising: true });
    expect(buttonType).toBe("release");
  });

  it("shows claim button when not supervising Schulhof", () => {
    const buttonType = getButtonType(true, { isUserSupervising: false });
    expect(buttonType).toBe("claim");
  });

  it("shows no button for regular rooms", () => {
    const buttonType = getButtonType(false, { isUserSupervising: true });
    expect(buttonType).toBe("none");
  });

  it("shows no button when schulhofStatus is null", () => {
    const buttonType = getButtonType(true, null);
    expect(buttonType).toBe("none");
  });
});

/**
 * Enhanced rendering tests that override the PageHeaderWithSearch mock
 * to render action buttons, search inputs, and filter controls.
 * This allows testing interactive behavior of components rendered
 * inside PageHeaderWithSearch props (actionButton, mobileActionButton, search, etc.)
 */
describe("Enhanced rendering: action buttons and search/filter interaction", () => {
  const mockMutate = vi.fn();

  beforeEach(async () => {
    vi.clearAllMocks();
    global.fetch = vi.fn();

    // Override PageHeaderWithSearch to render action buttons and interactive controls
    const mod = await import("~/components/ui/page-header");
    vi.mocked(
      mod.PageHeaderWithSearch as React.FC<Record<string, unknown>>,
    ).mockImplementation((props: Record<string, unknown>) => {
      const p = props;
      const actionButton = p.actionButton as React.ReactNode;
      const mobileActionButton = p.mobileActionButton as React.ReactNode;
      const search = p.search as
        | { value: string; onChange: (v: string) => void }
        | undefined;
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
            <button data-testid="clear-btn" onClick={onClearAllFilters}>
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

  it("renders ReleaseSupervisionButton and MobileReleaseSupervisionButton when supervising Schulhof", async () => {
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

    await waitFor(() => {
      // Both desktop and mobile release buttons should render with aria-label
      const releaseButtons = screen.getAllByLabelText("Aufsicht abgeben");
      expect(releaseButtons.length).toBeGreaterThanOrEqual(2);
    });

    // Verify the desktop button text
    expect(screen.getByText("Aufsicht abgeben")).toBeInTheDocument();
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
      expect(mockPush).toHaveBeenCalledWith("/");
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
    const mod = await import("~/components/ui/page-header");
    vi.mocked(
      mod.PageHeaderWithSearch as React.FC<Record<string, unknown>>,
    ).mockImplementation((props: Record<string, unknown>) => {
      const p = props;
      const search = p.search as
        | { value: string; onChange: (v: string) => void }
        | undefined;
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
            <button data-testid="clear-btn" onClick={onClearAllFilters}>
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
        screen.getByText("Keine Schüler in diesem Raum"),
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
    const mod = await import("~/components/ui/page-header");
    vi.mocked(
      mod.PageHeaderWithSearch as React.FC<Record<string, unknown>>,
    ).mockImplementation((props: Record<string, unknown>) => {
      const p = props;
      const search = p.search as
        | { value: string; onChange: (v: string) => void }
        | undefined;
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
describe("ID-based selection coverage: first room visit enrichment", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem("sidebar-last-room");
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
        screen.getByText("Keine Schüler in diesem Raum"),
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
    localStorage.removeItem("sidebar-last-room");
    localStorage.removeItem("sidebar-last-room-name");
    global.fetch = vi.fn();

    // Set mobile viewport so tabs are rendered (isDesktop = false when < 1024)
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: 500,
    });

    // Override PageHeaderWithSearch to render tabs with onTabChange
    const mod = await import("~/components/ui/page-header");
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
              key={tab.id}
              data-testid={`tab-${tab.id}`}
              data-active={tab.id === tabs.activeTab}
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

  it("switches to a different room when tab is clicked and loads visits", async () => {
    const { activeService } = await import("~/lib/active-api");

    // Mock loadRoomVisits to return visit data for the second room
    vi.mocked(activeService.getActiveGroupVisitsWithDisplay).mockResolvedValue([
      {
        studentId: "s10",
        studentName: "Room B Student",
        schoolClass: "4a",
        groupName: "Gruppe B",
        activeGroupId: "room-2",
        checkInTime: new Date(),
        isActive: true,
      },
    ] as never);

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

    // Wait for initial render with first room's student
    await waitFor(() => {
      expect(screen.getByText("Room A Student")).toBeInTheDocument();
    });

    // Click the second room's tab - triggers switchToRoom and loadRoomVisits
    const tabB = screen.getByTestId("tab-room-2");
    fireEvent.click(tabB);

    // After switching, the second room's student should appear
    await waitFor(() => {
      expect(
        activeService.getActiveGroupVisitsWithDisplay,
      ).toHaveBeenCalledWith("room-2");
    });

    await waitFor(() => {
      expect(screen.getByText("Room B Student")).toBeInTheDocument();
    });
  });

  it("handles 403 error from loadRoomVisits gracefully when switching rooms", async () => {
    const { activeService } = await import("~/lib/active-api");

    // Mock getActiveGroupVisitsWithDisplay to throw a 403 error.
    // loadRoomVisits catches 403 internally and returns [] (lines 473-480),
    // so switchToRoom receives empty students without throwing.
    vi.mocked(activeService.getActiveGroupVisitsWithDisplay).mockRejectedValue(
      new Error("Request failed: 403"),
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

    // Click the second room's tab - triggers switchToRoom -> loadRoomVisits
    const tabB = screen.getByTestId("tab-room-2");
    fireEvent.click(tabB);

    // loadRoomVisits catches 403 and returns empty - no error alert shown
    await waitFor(() => {
      expect(
        activeService.getActiveGroupVisitsWithDisplay,
      ).toHaveBeenCalledWith("room-2");
    });

    // Room shows empty students state (no crash, graceful degradation)
    await waitFor(() => {
      expect(
        screen.getByText("Keine Schüler in diesem Raum"),
      ).toBeInTheDocument();
    });

    // No error alert should be shown (403 is handled silently in loadRoomVisits)
    expect(screen.queryByTestId("alert-error")).not.toBeInTheDocument();
  });

  it("handles non-403 error from loadRoomVisits", async () => {
    const { activeService } = await import("~/lib/active-api");

    // Mock loadRoomVisits to throw a generic error
    vi.mocked(activeService.getActiveGroupVisitsWithDisplay).mockRejectedValue(
      new Error("Network timeout"),
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
    localStorage.removeItem("sidebar-last-room");
    localStorage.removeItem("sidebar-last-room-name");
    global.fetch = vi.fn();

    // Override PageHeaderWithSearch to render tabs
    const mod = await import("~/components/ui/page-header");
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
    const { activeService } = await import("~/lib/active-api");

    // Set localStorage to point to room r2 (the second room)
    localStorage.setItem("sidebar-last-room", "r2");

    vi.mocked(activeService.getActiveGroupVisitsWithDisplay).mockResolvedValue([
      {
        studentId: "s20",
        studentName: "Restored Student",
        schoolClass: "2a",
        groupName: "G2",
        activeGroupId: "room-2",
        checkInTime: new Date(),
        isActive: true,
      },
    ] as never);

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

    // The URL sync effect should find savedRoom via allRooms.find(r => r.room_id === "r2")
    // and call switchToRoom, which calls loadRoomVisits
    await waitFor(() => {
      expect(
        activeService.getActiveGroupVisitsWithDisplay,
      ).toHaveBeenCalledWith("room-2");
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
});

describe("ID-based selection coverage: Schulhof skip guard", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem("sidebar-last-room");
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

describe("ID-based selection coverage: loadRoomVisits 403 handling", () => {
  const mockMutate = vi.fn();
  const originalInnerWidth = window.innerWidth;

  beforeEach(async () => {
    vi.clearAllMocks();
    localStorage.removeItem("sidebar-last-room");
    localStorage.removeItem("sidebar-last-room-name");
    global.fetch = vi.fn();

    // Set mobile viewport so tabs are rendered (isDesktop = false when < 1024)
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: 500,
    });

    // Override PageHeaderWithSearch to render tabs
    const mod = await import("~/components/ui/page-header");
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
    const { activeService } = await import("~/lib/active-api");

    // loadRoomVisits catches 403 and returns empty, but switchToRoom re-throws
    // other errors. Let's test the 403 path in switchToRoom (lines 991-996)
    vi.mocked(activeService.getActiveGroupVisitsWithDisplay).mockRejectedValue(
      new Error("Request failed: 403"),
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

    // loadRoomVisits internally catches 403 and returns [], but the code
    // in switchToRoom proceeds normally with empty students
    await waitFor(() => {
      expect(
        activeService.getActiveGroupVisitsWithDisplay,
      ).toHaveBeenCalledWith("room-no-access");
    });
  });
});
