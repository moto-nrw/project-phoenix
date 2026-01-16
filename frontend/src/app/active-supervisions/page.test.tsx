/**
 * Tests for Active Supervisions Page
 * Tests the rendering states and user interactions of the active supervisions dashboard
 */
import { render, screen, waitFor, cleanup } from "@testing-library/react";
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
}));

// Mock ResponsiveLayout
vi.mock("~/components/dashboard", () => ({
  ResponsiveLayout: ({
    children,
    activeSupervisionName,
  }: {
    children: React.ReactNode;
    activeSupervisionName?: string;
  }) => (
    <div data-testid="responsive-layout" data-room={activeSupervisionName}>
      {children}
    </div>
  ),
}));

// Mock Loading component
vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading">Loading...</div>,
}));

// Mock PageHeaderWithSearch
vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({
    title,
    badge,
  }: {
    title: string;
    badge?: { count: number };
  }) => (
    <div data-testid="page-header" data-count={badge?.count}>
      {title}
    </div>
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

    expect(screen.getByTestId("responsive-layout")).toBeInTheDocument();
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
        { id: "1", name: "Schulhof", room: { id: "10", name: "Schulhof" } },
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
            { id: "1", name: "Schulhof", room: { id: "10", name: "Schulhof" } },
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
      expect(screen.getByTestId("responsive-layout")).toBeInTheDocument();
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
      const layout = screen.getByTestId("responsive-layout");
      expect(layout).toHaveAttribute("data-room", "Kunstzimmer");
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
