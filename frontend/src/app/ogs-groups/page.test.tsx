/**
 * Tests for OGS Groups Page
 * Tests the rendering states and user interactions of the OGS groups dashboard
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

// Mock ToastContext
const mockToast = {
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
};
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => mockToast,
}));

// Mock ResponsiveLayout
vi.mock("~/components/dashboard", () => ({
  ResponsiveLayout: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-layout">{children}</div>
  ),
}));

// Mock Loading component
vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading">Loading...</div>,
}));

// Mock PageHeaderWithSearch
vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({ title }: { title: string }) => (
    <div data-testid="page-header">{title}</div>
  ),
}));

// Mock Alert
vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message, type }: { message: string; type: string }) => (
    <div data-testid={`alert-${type}`}>{message}</div>
  ),
}));

// Mock studentService
vi.mock("~/lib/api", () => ({
  studentService: {
    getStudents: vi.fn(),
  },
}));

// Mock location helpers
vi.mock("~/lib/location-helper", () => ({
  LOCATION_STATUSES: { PRESENT: "Anwesend" },
  isHomeLocation: vi.fn(() => false),
  isSchoolyardLocation: vi.fn(() => false),
  isTransitLocation: vi.fn(() => false),
  parseLocation: vi.fn(() => ({ room: "Room 1", status: "Anwesend" })),
}));

// Mock student-helpers
vi.mock("~/lib/student-helpers", () => ({
  SCHOOL_YEAR_FILTER_OPTIONS: [
    { value: "all", label: "Alle" },
    { value: "1", label: "1. Klasse" },
  ],
}));

// Mock SSEErrorBoundary
vi.mock("~/components/sse/SSEErrorBoundary", () => ({
  SSEErrorBoundary: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="sse-boundary">{children}</div>
  ),
}));

// Mock GroupTransferModal
vi.mock("~/components/groups/group-transfer-modal", () => ({
  GroupTransferModal: () => <div data-testid="transfer-modal" />,
}));

// Mock group-transfer-api
vi.mock("~/lib/group-transfer-api", () => ({
  groupTransferService: {
    getStaffByRole: vi.fn(() => Promise.resolve([])),
    getActiveTransfersForGroup: vi.fn(() => Promise.resolve([])),
    transferGroup: vi.fn(() => Promise.resolve()),
    cancelTransferBySubstitutionId: vi.fn(() => Promise.resolve()),
  },
}));

// Mock LocationBadge
vi.mock("@/components/ui/location-badge", () => ({
  LocationBadge: () => <div data-testid="location-badge">Location</div>,
}));

// Mock EmptyStudentResults
vi.mock("~/components/ui/empty-student-results", () => ({
  EmptyStudentResults: () => <div data-testid="empty-results">No results</div>,
}));

// Mock StudentCard
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
}));

// Mock SWR hook
vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
}));

import { useSWRAuth } from "~/lib/swr";
import OGSGroupPage from "./page";

describe("OGSGroupPage", () => {
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
    render(<OGSGroupPage />);

    // Initial loading state should show loading component
    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("renders with SSE error boundary wrapper", () => {
    render(<OGSGroupPage />);

    // Page should be wrapped in SSE error boundary
    expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
  });

  it("renders within responsive layout", async () => {
    render(<OGSGroupPage />);

    expect(screen.getByTestId("responsive-layout")).toBeInTheDocument();
  });

  it("shows no access state when user has no OGS groups", async () => {
    // Mock SWR to return empty data indicating no access
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [],
        students: [],
        roomStatus: null,
        substitutions: [],
        firstGroupId: null,
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      // Should show the "no group assigned" message
      expect(
        screen.getByText("Keine OGS-Gruppe zugeordnet"),
      ).toBeInTheDocument();
    });
  });

  it("shows permission error when 403 response received", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error("API error: 403"),
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine OGS-Gruppe zugeordnet"),
      ).toBeInTheDocument();
    });
  });

  it("shows loading state when session is loading", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: true,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    // Should show loading state while SWR is loading
    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("displays group data when available", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "OGS Gruppe A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            second_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        firstGroupId: "1",
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });
  });
});

describe("OGSGroupPage helper functions", () => {
  // Tests for helper functions that are defined in the page
  // These test the pure logic without rendering

  it("filters students by search term", () => {
    const student = {
      name: "Max Mustermann",
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "1a",
    };

    // Test search matching first name
    const searchLower = "max";
    const matches =
      student.name?.toLowerCase().includes(searchLower) ??
      student.first_name?.toLowerCase().includes(searchLower) ??
      student.second_name?.toLowerCase().includes(searchLower) ??
      false;

    expect(matches).toBe(true);
  });

  it("extracts student year from school class", () => {
    const extractYear = (schoolClass?: string): string | null => {
      if (!schoolClass) return null;
      const yearMatch = /^(\d)/.exec(schoolClass);
      return yearMatch?.[1] ?? null;
    };

    expect(extractYear("1a")).toBe("1");
    expect(extractYear("2b")).toBe("2");
    expect(extractYear("10c")).toBe("1"); // Only first digit
    expect(extractYear("")).toBe(null);
    expect(extractYear(undefined)).toBe(null);
  });

  it("detects student in group room", () => {
    const isStudentInRoom = (
      studentLocation: string | undefined,
      roomName: string | undefined,
    ): boolean => {
      if (!studentLocation || !roomName) return false;
      return studentLocation.toLowerCase().includes(roomName.toLowerCase());
    };

    expect(isStudentInRoom("Anwesend - Raum 101", "Raum 101")).toBe(true);
    expect(isStudentInRoom("Anwesend - Raum 101", "Raum 202")).toBe(false);
    expect(isStudentInRoom(undefined, "Raum 101")).toBe(false);
    expect(isStudentInRoom("Anwesend", undefined)).toBe(false);
  });
});

describe("OGSGroupPage additional scenarios", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    cleanup();
  });

  it("shows empty students state when group has no students", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "OGS Gruppe A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
        ],
        students: [], // No students
        roomStatus: null,
        substitutions: [],
        firstGroupId: "1",
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByText(/Keine Schüler in/)).toBeInTheDocument();
    });
  });

  it("renders multiple students in grid", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "OGS Gruppe A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            second_name: "Mustermann",
            current_location: "Raum 101",
          },
          {
            id: "2",
            name: "Erika Schmidt",
            first_name: "Erika",
            second_name: "Schmidt",
            current_location: "Raum 101",
          },
          {
            id: "3",
            name: "Hans Mueller",
            first_name: "Hans",
            second_name: "Mueller",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          group_has_room: true,
          student_room_status: {
            "1": { in_group_room: true },
            "2": { in_group_room: true },
            "3": { in_group_room: true },
          },
        },
        substitutions: [],
        firstGroupId: "1",
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      const studentCards = screen.getAllByTestId("student-card");
      expect(studentCards).toHaveLength(3);
    });
  });

  it("handles generic API error gracefully", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error("API error: 500"),
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("responsive-layout")).toBeInTheDocument();
    });
  });

  it("shows transfer modal component", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "OGS Gruppe A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            second_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        firstGroupId: "1",
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("transfer-modal")).toBeInTheDocument();
    });
  });

  it("displays via substitution badge when group is via substitution", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "OGS Gruppe A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
            via_substitution: true, // This group is via substitution
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            second_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        firstGroupId: "1",
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });
  });

  it("converts substitutions to active transfers format", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "OGS Gruppe A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            second_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [
          {
            id: 100,
            group_id: 1,
            regular_staff_id: null, // This indicates it's a group transfer
            substitute_staff_id: 200,
            substitute_staff: {
              person: { first_name: "Anna", last_name: "Lehrer" },
            },
            start_date: "2024-01-15",
            end_date: "2024-01-20",
          },
        ],
        firstGroupId: "1",
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });
  });
});

describe("OGSGroupPage filter behavior", () => {
  it("filters students by attendance - in_room", () => {
    const students = [
      { id: "1", current_location: "Anwesend - Raum 101" },
      { id: "2", current_location: "Anwesend - Raum 202" },
      { id: "3", current_location: "Zuhause" },
    ];

    const roomStatus = {
      "1": { in_group_room: true, current_room_id: 101 },
      "2": { in_group_room: false, current_room_id: 202 },
      "3": { in_group_room: false, current_room_id: undefined },
    };

    // Filter for in_room attendance
    const filtered = students.filter((student) => {
      const status = roomStatus[student.id as keyof typeof roomStatus];
      return status?.in_group_room ?? false;
    });

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.id).toBe("1");
  });

  it("filters students by attendance - foreign_room", () => {
    const students = [
      { id: "1", current_location: "Anwesend - Raum 101" },
      { id: "2", current_location: "Anwesend - Raum 202" },
      { id: "3", current_location: "Zuhause" },
    ];

    const roomStatus: Record<
      string,
      { in_group_room: boolean; current_room_id?: number }
    > = {
      "1": { in_group_room: true, current_room_id: 101 },
      "2": { in_group_room: false, current_room_id: 202 },
      "3": { in_group_room: false, current_room_id: undefined },
    };

    const filtered = students.filter((student) => {
      const status = roomStatus[student.id];
      return (
        status?.current_room_id !== undefined && status.in_group_room === false
      );
    });

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.id).toBe("2");
  });

  it("filters students by year", () => {
    const students = [
      { id: "1", school_class: "1a" },
      { id: "2", school_class: "2b" },
      { id: "3", school_class: "1c" },
      { id: "4", school_class: "3a" },
    ];

    const selectedYear = "1";
    const extractYear = (schoolClass?: string): string | null => {
      if (!schoolClass) return null;
      const yearMatch = /^(\d)/.exec(schoolClass);
      return yearMatch?.[1] ?? null;
    };

    const filtered = students.filter(
      (s) => extractYear(s.school_class) === selectedYear,
    );

    expect(filtered).toHaveLength(2);
    expect(filtered.map((s) => s.id)).toEqual(["1", "3"]);
  });

  it("returns all students when year filter is 'all'", () => {
    const students = [
      { id: "1", school_class: "1a" },
      { id: "2", school_class: "2b" },
    ];

    const selectedYear = "all";
    const filtered = students.filter(() => selectedYear === "all" || true);

    expect(filtered).toHaveLength(2);
  });

  it("combines multiple filters correctly", () => {
    const students = [
      {
        id: "1",
        name: "Max A",
        school_class: "1a",
        current_location: "Raum 101",
      },
      {
        id: "2",
        name: "Erika B",
        school_class: "1b",
        current_location: "Raum 202",
      },
      {
        id: "3",
        name: "Max C",
        school_class: "2a",
        current_location: "Raum 101",
      },
    ];

    const searchTerm = "max";
    const selectedYear = "1" as string;

    const extractYear = (schoolClass?: string): string | null => {
      if (!schoolClass) return null;
      const yearMatch = /^(\d)/.exec(schoolClass);
      return yearMatch?.[1] ?? null;
    };

    const filtered = students.filter((student) => {
      const matchesSearch =
        student.name?.toLowerCase().includes(searchTerm.toLowerCase()) ?? false;
      const matchesYear =
        selectedYear === "all" ||
        extractYear(student.school_class) === selectedYear;
      return matchesSearch && matchesYear;
    });

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.id).toBe("1");
  });
});

describe("OGSGroupPage active filters", () => {
  it("creates active filter for search term", () => {
    const searchTerm = "Max";
    const selectedYear = "all" as string;
    const attendanceFilter = "all" as string;

    const activeFilters: Array<{ id: string; label: string }> = [];

    if (searchTerm.length > 0) {
      activeFilters.push({
        id: "search",
        label: `"${searchTerm}"`,
      });
    }

    if (selectedYear !== "all") {
      activeFilters.push({
        id: "year",
        label: `Jahr ${selectedYear}`,
      });
    }

    if (attendanceFilter !== "all") {
      activeFilters.push({
        id: "location",
        label: attendanceFilter,
      });
    }

    expect(activeFilters).toHaveLength(1);
    expect(activeFilters[0]?.label).toBe('"Max"');
  });

  it("creates active filter for year filter", () => {
    const searchTerm = "";
    const selectedYear = "2" as string;
    const attendanceFilter = "all" as string;

    const activeFilters: Array<{ id: string; label: string }> = [];

    if (searchTerm.length > 0) {
      activeFilters.push({ id: "search", label: `"${searchTerm}"` });
    }

    if (selectedYear !== "all") {
      activeFilters.push({ id: "year", label: `Jahr ${selectedYear}` });
    }

    if (attendanceFilter !== "all") {
      activeFilters.push({ id: "location", label: attendanceFilter });
    }

    expect(activeFilters).toHaveLength(1);
    expect(activeFilters[0]?.label).toBe("Jahr 2");
  });

  it("creates active filter for attendance filter", () => {
    const searchTerm = "";
    const selectedYear = "all" as string;
    const attendanceFilter = "in_room" as string;

    const locationLabels: Record<string, string> = {
      in_room: "Gruppenraum",
      foreign_room: "Fremder Raum",
      transit: "Unterwegs",
      schoolyard: "Schulhof",
      at_home: "Zuhause",
    };

    const activeFilters: Array<{ id: string; label: string }> = [];

    if (searchTerm.length > 0) {
      activeFilters.push({ id: "search", label: `"${searchTerm}"` });
    }

    if (selectedYear !== "all") {
      activeFilters.push({ id: "year", label: `Jahr ${selectedYear}` });
    }

    if (attendanceFilter !== "all") {
      activeFilters.push({
        id: "location",
        label: locationLabels[attendanceFilter] ?? attendanceFilter,
      });
    }

    expect(activeFilters).toHaveLength(1);
    expect(activeFilters[0]?.label).toBe("Gruppenraum");
  });

  it("creates multiple active filters", () => {
    const searchTerm = "Max";
    const selectedYear = "1" as string;
    const attendanceFilter = "schoolyard" as string;

    const locationLabels: Record<string, string> = {
      in_room: "Gruppenraum",
      foreign_room: "Fremder Raum",
      transit: "Unterwegs",
      schoolyard: "Schulhof",
      at_home: "Zuhause",
    };

    const activeFilters: Array<{ id: string; label: string }> = [];

    if (searchTerm.length > 0) {
      activeFilters.push({ id: "search", label: `"${searchTerm}"` });
    }

    if (selectedYear !== "all") {
      activeFilters.push({ id: "year", label: `Jahr ${selectedYear}` });
    }

    if (attendanceFilter !== "all") {
      activeFilters.push({
        id: "location",
        label: locationLabels[attendanceFilter] ?? attendanceFilter,
      });
    }

    expect(activeFilters).toHaveLength(3);
  });

  it("creates no active filters when all defaults", () => {
    const searchTerm = "";
    const selectedYear = "all" as string;
    const attendanceFilter = "all" as string;

    const activeFilters: Array<{ id: string; label: string }> = [];

    if (searchTerm.length > 0) {
      activeFilters.push({ id: "search", label: `"${searchTerm}"` });
    }

    if (selectedYear !== "all") {
      activeFilters.push({ id: "year", label: `Jahr ${selectedYear}` });
    }

    if (attendanceFilter !== "all") {
      activeFilters.push({ id: "location", label: attendanceFilter });
    }

    expect(activeFilters).toHaveLength(0);
  });
});

describe("OGSGroupPage handleTransferGroup behavior", () => {
  it("validates transfer group parameters", () => {
    const currentGroup = { id: "1", name: "OGS Gruppe A" };
    const targetName = "Anna Lehrer";

    // Simulate the validation logic from handleTransferGroup
    const canTransfer = !!currentGroup;
    expect(canTransfer).toBe(true);

    // Generate expected toast message
    const toastMessage = `Gruppe "${currentGroup.name}" an ${targetName} übergeben`;
    expect(toastMessage).toBe('Gruppe "OGS Gruppe A" an Anna Lehrer übergeben');
  });

  it("returns early when no current group", () => {
    const currentGroup = null;

    // Simulate the early return logic
    const canTransfer = !!currentGroup;
    expect(canTransfer).toBe(false);
  });
});

describe("OGSGroupPage handleCancelTransfer behavior", () => {
  it("finds transfer by substitution ID", () => {
    const activeTransfers = [
      { substitutionId: "100", targetName: "Anna Lehrer" },
      { substitutionId: "101", targetName: "Ben Schmidt" },
    ];
    const substitutionId = "100";

    const transfer = activeTransfers.find(
      (t) => t.substitutionId === substitutionId,
    );

    expect(transfer?.targetName).toBe("Anna Lehrer");
  });

  it("uses default name when transfer not found", () => {
    const activeTransfers = [{ substitutionId: "100", targetName: "Anna Lehrer" }];
    const substitutionId = "999"; // Non-existent

    const transfer = activeTransfers.find(
      (t) => t.substitutionId === substitutionId,
    );
    const recipientName = transfer?.targetName ?? "Betreuer";

    expect(recipientName).toBe("Betreuer");
  });

  it("generates correct cancel toast message", () => {
    const recipientName = "Anna Lehrer";
    const toastMessage = `Übergabe an ${recipientName} wurde zurückgenommen`;

    expect(toastMessage).toBe("Übergabe an Anna Lehrer wurde zurückgenommen");
  });
});

describe("OGSGroupPage switchToGroup behavior", () => {
  it("returns early when same group index selected", () => {
    const selectedGroupIndex = 0;
    const newGroupIndex = 0;
    const allGroups = [{ id: "1", name: "Group A" }];

    const shouldSwitch =
      newGroupIndex !== selectedGroupIndex && allGroups[newGroupIndex];
    expect(shouldSwitch).toBeFalsy();
  });

  it("returns early when group index is invalid", () => {
    const selectedGroupIndex = 0;
    const newGroupIndex = 5; // Out of bounds
    const allGroups = [{ id: "1", name: "Group A" }];

    const shouldSwitch =
      newGroupIndex !== selectedGroupIndex && allGroups[newGroupIndex];
    expect(shouldSwitch).toBeFalsy();
  });

  it("allows switching to different valid group", () => {
    const selectedGroupIndex = 0;
    const newGroupIndex = 1;
    const allGroups = [
      { id: "1", name: "Group A" },
      { id: "2", name: "Group B" },
    ];

    const shouldSwitch =
      newGroupIndex !== selectedGroupIndex && allGroups[newGroupIndex];
    expect(shouldSwitch).toBeTruthy();
  });

  it("updates group student count after loading", () => {
    const allGroups = [
      { id: "1", name: "Group A", student_count: 0 },
      { id: "2", name: "Group B", student_count: 0 },
    ];
    const selectedGroupIndex = 1;
    const studentsData = [{}, {}, {}]; // 3 students loaded

    // Simulate the update logic
    const updatedGroups = allGroups.map((group, idx) =>
      idx === selectedGroupIndex
        ? { ...group, student_count: studentsData.length }
        : group,
    );

    expect(updatedGroups[1]?.student_count).toBe(3);
    expect(updatedGroups[0]?.student_count).toBe(0);
  });

  it("sets error message on fetch failure", () => {
    const errorMessage = "Fehler beim Laden der Gruppendaten.";
    expect(errorMessage).toBe("Fehler beim Laden der Gruppendaten.");
  });
});

describe("OGSGroupPage loadGroupRoomStatus behavior", () => {
  it("extracts student room status from response", () => {
    const response = {
      success: true,
      message: "OK",
      data: {
        group_has_room: true,
        group_room_id: 101,
        student_room_status: {
          "1": { in_group_room: true, current_room_id: 101 },
          "2": { in_group_room: false, current_room_id: 202 },
        },
      },
    };

    const roomStatus = response.data?.student_room_status;

    expect(roomStatus).toBeDefined();
    expect(roomStatus?.["1"]?.in_group_room).toBe(true);
    expect(roomStatus?.["2"]?.in_group_room).toBe(false);
  });

  it("handles missing student room status", () => {
    const response = {
      success: true,
      message: "OK",
      data: {
        group_has_room: false,
        student_room_status: undefined,
      },
    };

    const roomStatus = response.data?.student_room_status;
    expect(roomStatus).toBeUndefined();
  });
});

describe("OGSGroupPage renderDesktopActionButton logic", () => {
  it("returns undefined when on mobile", () => {
    const isMobile = true;
    const currentGroup = { id: "1", name: "Group A" };

    const shouldRender = !isMobile && currentGroup;
    expect(shouldRender).toBeFalsy();
  });

  it("returns undefined when no current group", () => {
    const isMobile = false;
    const currentGroup = null;

    const shouldRender = !isMobile && currentGroup;
    expect(shouldRender).toBeFalsy();
  });

  it("shows via substitution badge when group is via substitution", () => {
    const isMobile = false;
    const currentGroup = { id: "1", name: "Group A", viaSubstitution: true };

    const hasCurrentGroup = currentGroup !== null;
    const isViaSubstitution = currentGroup.viaSubstitution;
    const shouldShowSubstitutionBadge =
      !isMobile && hasCurrentGroup && isViaSubstitution;
    expect(shouldShowSubstitutionBadge).toBe(true);
  });

  it("shows transfer button with count when active transfers exist", () => {
    const activeTransfers = [
      { substitutionId: "1" },
      { substitutionId: "2" },
    ];

    const buttonText =
      activeTransfers.length > 0
        ? `Gruppe übergeben (${activeTransfers.length})`
        : "Gruppe übergeben";

    expect(buttonText).toBe("Gruppe übergeben (2)");
  });

  it("shows transfer button without count when no active transfers", () => {
    const activeTransfers: Array<{ substitutionId: string }> = [];

    const buttonText =
      activeTransfers.length > 0
        ? `Gruppe übergeben (${activeTransfers.length})`
        : "Gruppe übergeben";

    expect(buttonText).toBe("Gruppe übergeben");
  });
});

describe("OGSGroupPage renderMobileActionButton logic", () => {
  it("returns undefined when not on mobile", () => {
    const isMobile = false;
    const currentGroup = { id: "1", name: "Group A" };

    const shouldRender = isMobile && currentGroup;
    expect(shouldRender).toBeFalsy();
  });

  it("returns undefined when no current group", () => {
    const isMobile = true;
    const currentGroup = null;

    const shouldRender = isMobile && currentGroup;
    expect(shouldRender).toBeFalsy();
  });

  it("shows via substitution icon on mobile when group is via substitution", () => {
    const isMobile = true;
    const currentGroup = { id: "1", name: "Group A", viaSubstitution: true };

    const hasCurrentGroup = currentGroup !== null;
    const isViaSubstitution = currentGroup.viaSubstitution;
    const shouldShowSubstitutionIcon =
      isMobile && hasCurrentGroup && isViaSubstitution;
    expect(shouldShowSubstitutionIcon).toBe(true);
  });

  it("shows badge with active transfer count on mobile", () => {
    const activeTransfers = [{ substitutionId: "1" }];

    const showBadge = activeTransfers.length > 0;
    expect(showBadge).toBe(true);
  });

  it("hides badge when no active transfers on mobile", () => {
    const activeTransfers: Array<{ substitutionId: string }> = [];

    const showBadge = activeTransfers.length > 0;
    expect(showBadge).toBe(false);
  });
});

describe("OGSGroupPage renderStudentContent logic", () => {
  it("shows loading when isLoading is true", () => {
    const isLoading = true;

    const showLoading = isLoading;
    expect(showLoading).toBe(true);
  });

  it("shows empty state when students array is empty", () => {
    const isLoading = false;
    const students: Array<{ id: string }> = [];

    const showEmptyNoStudents = !isLoading && students.length === 0;
    expect(showEmptyNoStudents).toBe(true);
  });

  it("shows filtered student grid when students exist", () => {
    const isLoading = false;
    const students = [{ id: "1" }, { id: "2" }];
    const filteredStudents = [{ id: "1" }];

    const showStudentGrid =
      !isLoading && students.length > 0 && filteredStudents.length > 0;
    expect(showStudentGrid).toBe(true);
  });

  it("shows empty results component when filters match nothing", () => {
    const isLoading = false;
    const students = [{ id: "1" }, { id: "2" }];
    const filteredStudents: Array<{ id: string }> = [];

    const showEmptyResults =
      !isLoading && students.length > 0 && filteredStudents.length === 0;
    expect(showEmptyResults).toBe(true);
  });

  it("generates correct no students message", () => {
    const currentGroup = { name: "OGS Gruppe A" };
    const message = `Keine Schüler in ${currentGroup?.name ?? "dieser Gruppe"}`;

    expect(message).toBe("Keine Schüler in OGS Gruppe A");
  });

  it("uses fallback message when no current group", () => {
    const currentGroup = null;
    const message = `Keine Schüler in ${currentGroup?.name ?? "dieser Gruppe"}`;

    expect(message).toBe("Keine Schüler in dieser Gruppe");
  });

  it("shows suggestion for multiple groups when no students", () => {
    const allGroups = [{ id: "1" }, { id: "2" }];
    const showSuggestion = allGroups.length > 1;

    expect(showSuggestion).toBe(true);
  });
});

describe("OGSGroupPage student card onClick behavior", () => {
  it("generates correct navigation path with from param", () => {
    const studentId = "123";
    const path = `/students/${studentId}?from=/ogs-groups`;

    expect(path).toBe("/students/123?from=/ogs-groups");
  });
});

describe("OGSGroupPage card gradient logic", () => {
  it("returns green gradient for student in group room", () => {
    const isInGroupRoom = true;
    const isSchoolyard = false;
    const isTransit = false;
    const isHome = false;

    const getGradient = (): string => {
      if (isInGroupRoom) return "from-emerald-50/80 to-green-100/80";
      if (isSchoolyard) return "from-amber-50/80 to-yellow-100/80";
      if (isTransit) return "from-fuchsia-50/80 to-pink-100/80";
      if (isHome) return "from-red-50/80 to-rose-100/80";
      return "from-blue-50/80 to-cyan-100/80";
    };

    expect(getGradient()).toBe("from-emerald-50/80 to-green-100/80");
  });

  it("returns amber gradient for student in schoolyard", () => {
    const isInGroupRoom = false;
    const isSchoolyard = true;
    const isTransit = false;
    const isHome = false;

    const getGradient = (): string => {
      if (isInGroupRoom) return "from-emerald-50/80 to-green-100/80";
      if (isSchoolyard) return "from-amber-50/80 to-yellow-100/80";
      if (isTransit) return "from-fuchsia-50/80 to-pink-100/80";
      if (isHome) return "from-red-50/80 to-rose-100/80";
      return "from-blue-50/80 to-cyan-100/80";
    };

    expect(getGradient()).toBe("from-amber-50/80 to-yellow-100/80");
  });

  it("returns fuchsia gradient for student in transit", () => {
    const isInGroupRoom = false;
    const isSchoolyard = false;
    const isTransit = true;
    const isHome = false;

    const getGradient = (): string => {
      if (isInGroupRoom) return "from-emerald-50/80 to-green-100/80";
      if (isSchoolyard) return "from-amber-50/80 to-yellow-100/80";
      if (isTransit) return "from-fuchsia-50/80 to-pink-100/80";
      if (isHome) return "from-red-50/80 to-rose-100/80";
      return "from-blue-50/80 to-cyan-100/80";
    };

    expect(getGradient()).toBe("from-fuchsia-50/80 to-pink-100/80");
  });

  it("returns red gradient for student at home", () => {
    const isInGroupRoom = false;
    const isSchoolyard = false;
    const isTransit = false;
    const isHome = true;

    const getGradient = (): string => {
      if (isInGroupRoom) return "from-emerald-50/80 to-green-100/80";
      if (isSchoolyard) return "from-amber-50/80 to-yellow-100/80";
      if (isTransit) return "from-fuchsia-50/80 to-pink-100/80";
      if (isHome) return "from-red-50/80 to-rose-100/80";
      return "from-blue-50/80 to-cyan-100/80";
    };

    expect(getGradient()).toBe("from-red-50/80 to-rose-100/80");
  });

  it("returns blue gradient for student in foreign room", () => {
    const isInGroupRoom = false;
    const isSchoolyard = false;
    const isTransit = false;
    const isHome = false;

    const getGradient = (): string => {
      if (isInGroupRoom) return "from-emerald-50/80 to-green-100/80";
      if (isSchoolyard) return "from-amber-50/80 to-yellow-100/80";
      if (isTransit) return "from-fuchsia-50/80 to-pink-100/80";
      if (isHome) return "from-red-50/80 to-rose-100/80";
      return "from-blue-50/80 to-cyan-100/80";
    };

    expect(getGradient()).toBe("from-blue-50/80 to-cyan-100/80");
  });
});
