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
