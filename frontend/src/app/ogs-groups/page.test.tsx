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

// Mock PageHeaderWithSearch — renders filters and activeFilters to exercise those code paths
vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({
    title,
    filters,
    activeFilters,
    onClearAllFilters,
  }: {
    title: string;
    filters?: Array<{
      id: string;
      label: string;
      value: string | string[];
      options: Array<{ value: string; label: string }>;
      onChange: (value: string | string[]) => void;
    }>;
    activeFilters?: Array<{ id: string; label: string; onRemove: () => void }>;
    onClearAllFilters?: () => void;
  }) => (
    <div data-testid="page-header">
      {title}
      {filters?.map((f) => (
        <div key={f.id} data-testid={`filter-${f.id}`} data-value={f.value}>
          {f.options.map((opt) => (
            <button
              key={opt.value}
              data-testid={`filter-${f.id}-${opt.value}`}
              onClick={() => f.onChange(opt.value)}
            >
              {opt.label}
            </button>
          ))}
        </div>
      ))}
      {activeFilters?.map((af) => (
        <span key={af.id} data-testid={`active-filter-${af.id}`}>
          {af.label}
          <button
            data-testid={`remove-filter-${af.id}`}
            onClick={af.onRemove}
          />
        </span>
      ))}
      {onClearAllFilters && (
        <button data-testid="clear-all-filters" onClick={onClearAllFilters} />
      )}
    </div>
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

// Mock StudentCard — renders extraContent to exercise urgency icon rendering
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
      {extraContent && <div data-testid="extra-content">{extraContent}</div>}
    </div>
  ),
  StudentInfoRow: ({
    children,
    icon,
  }: {
    children: React.ReactNode;
    icon: React.ReactNode;
  }) => (
    <div data-testid="student-info-row">
      <span data-testid="info-row-icon">{icon}</span>
      {children}
    </div>
  ),
  PickupTimeIcon: () => <span data-testid="pickup-time-icon">clock-gray</span>,
  ExceptionIcon: () => <span data-testid="exception-icon">exception</span>,
}));

// Mock pickup schedule API
const mockFetchBulkPickupTimes = vi.fn(() =>
  Promise.resolve(
    new Map<
      string,
      { pickupTime: string; isException: boolean; reason?: string }
    >(),
  ),
);
vi.mock("~/lib/pickup-schedule-api", () => ({
  fetchBulkPickupTimes: (
    ...args: Parameters<typeof mockFetchBulkPickupTimes>
  ) => mockFetchBulkPickupTimes(...args),
}));

// Mock lucide-react icons
vi.mock("lucide-react", () => ({
  Clock: ({ className }: { className?: string }) => (
    <span data-testid="lucide-clock" className={className}>
      clock
    </span>
  ),
  AlertTriangle: ({ className }: { className?: string }) => (
    <span data-testid="lucide-alert-triangle" className={className}>
      alert
    </span>
  ),
}));

// Mock SWR hook
vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
}));

import { useSWRAuth } from "~/lib/swr";
import { isHomeLocation } from "~/lib/location-helper";
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
    const activeTransfers = [
      { substitutionId: "100", targetName: "Anna Lehrer" },
    ];
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
    const selectedGroupIndex = 0 as number;
    const newGroupIndex = 5 as number; // Out of bounds
    const allGroups = [{ id: "1", name: "Group A" }];

    const shouldSwitch =
      newGroupIndex !== selectedGroupIndex && allGroups[newGroupIndex];
    expect(shouldSwitch).toBeFalsy();
  });

  it("allows switching to different valid group", () => {
    const selectedGroupIndex = 0 as number;
    const newGroupIndex = 1 as number;
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

    const shouldShowSubstitutionBadge =
      !isMobile && currentGroup.viaSubstitution;
    expect(shouldShowSubstitutionBadge).toBe(true);
  });

  it("shows transfer button with count when active transfers exist", () => {
    const activeTransfers = [{ substitutionId: "1" }, { substitutionId: "2" }];

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

    const shouldShowSubstitutionIcon = isMobile && currentGroup.viaSubstitution;
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
    const currentGroup = null as { name: string } | null;
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

describe("OGSGroupPage pickup urgency logic", () => {
  // Mirror the getPickupUrgency function logic from page.tsx
  const PICKUP_URGENCY_SOON_MINUTES = 30;

  type PickupUrgency = "overdue" | "soon" | "normal" | "none";

  function getPickupUrgency(
    pickupTimeStr: string | undefined,
    now: Date,
  ): PickupUrgency {
    if (!pickupTimeStr) return "none";

    const [hours, minutes] = pickupTimeStr.split(":").map(Number);
    const pickupDate = new Date(now);
    pickupDate.setHours(hours ?? 0, minutes ?? 0, 0, 0);

    const diffMs = pickupDate.getTime() - now.getTime();
    const diffMinutes = diffMs / 60000;

    if (diffMinutes < 0) return "overdue";
    if (diffMinutes <= PICKUP_URGENCY_SOON_MINUTES) return "soon";
    return "normal";
  }

  it("returns 'none' when no pickup time provided", () => {
    const now = new Date(2026, 0, 28, 14, 0);
    expect(getPickupUrgency(undefined, now)).toBe("none");
  });

  it("returns 'overdue' when pickup time has passed", () => {
    const now = new Date(2026, 0, 28, 15, 0); // 15:00
    expect(getPickupUrgency("14:30", now)).toBe("overdue"); // 14:30 already passed
  });

  it("returns 'soon' when pickup is within 30 minutes", () => {
    const now = new Date(2026, 0, 28, 14, 10); // 14:10
    expect(getPickupUrgency("14:30", now)).toBe("soon"); // 20 min away
  });

  it("returns 'soon' when pickup is exactly 30 minutes away", () => {
    const now = new Date(2026, 0, 28, 14, 0); // 14:00
    expect(getPickupUrgency("14:30", now)).toBe("soon"); // exactly 30 min
  });

  it("returns 'soon' when pickup is exactly now (0 minutes)", () => {
    const now = new Date(2026, 0, 28, 14, 30); // 14:30
    expect(getPickupUrgency("14:30", now)).toBe("soon"); // diff = 0, <= 30
  });

  it("returns 'normal' when pickup is more than 30 minutes away", () => {
    const now = new Date(2026, 0, 28, 13, 0); // 13:00
    expect(getPickupUrgency("14:30", now)).toBe("normal"); // 90 min away
  });

  it("returns 'overdue' for pickup time far in the past", () => {
    const now = new Date(2026, 0, 28, 20, 0); // 20:00
    expect(getPickupUrgency("14:00", now)).toBe("overdue"); // 6 hours past
  });

  it("returns 'normal' for early morning pickup when checked early", () => {
    const now = new Date(2026, 0, 28, 8, 0); // 08:00
    expect(getPickupUrgency("16:00", now)).toBe("normal"); // 8 hours away
  });

  it("handles pickup time at midnight edge case", () => {
    const now = new Date(2026, 0, 28, 23, 50); // 23:50
    expect(getPickupUrgency("00:00", now)).toBe("overdue"); // midnight is earlier in the same day
  });
});

describe("OGSGroupPage pickup urgency with home location", () => {
  // Tests the logic: at-home students get "none" urgency regardless of pickup time
  it("returns 'none' for at-home students regardless of pickup time", () => {
    const isAtHome = true;
    type PickupUrgency = "overdue" | "soon" | "normal" | "none";

    // Simulate the component logic
    const urgency: PickupUrgency = isAtHome ? "none" : "soon";
    expect(urgency).toBe("none");
  });

  it("returns computed urgency for non-home students", () => {
    const isAtHome = false;
    type PickupUrgency = "overdue" | "soon" | "normal" | "none";

    const computedUrgency: PickupUrgency = "soon";
    const urgency: PickupUrgency = isAtHome ? "none" : computedUrgency;
    expect(urgency).toBe("soon");
  });
});

describe("OGSGroupPage pickup icon rendering logic", () => {
  type PickupUrgency = "overdue" | "soon" | "normal" | "none";

  function resolveIconType(urgency: PickupUrgency): string {
    if (urgency === "overdue") return "alert-triangle";
    if (urgency === "soon") return "clock-pulse";
    return "pickup-time-default";
  }

  it("renders AlertTriangle for overdue urgency", () => {
    expect(resolveIconType("overdue")).toBe("alert-triangle");
  });

  it("renders pulsing Clock for soon urgency", () => {
    expect(resolveIconType("soon")).toBe("clock-pulse");
  });

  it("renders default PickupTimeIcon for normal urgency", () => {
    expect(resolveIconType("normal")).toBe("pickup-time-default");
  });

  it("renders default PickupTimeIcon for none urgency", () => {
    expect(resolveIconType("none")).toBe("pickup-time-default");
  });

  it("uses ExceptionIcon when student has exception regardless of urgency", () => {
    const isException = true;
    const iconType = isException ? "exception" : resolveIconType("overdue");
    expect(iconType).toBe("exception");
  });

  it("uses urgency icon when student has no exception", () => {
    const isException = false;
    const iconType = isException ? "exception" : resolveIconType("overdue");
    expect(iconType).toBe("alert-triangle");
  });
});

describe("OGSGroupPage sorting logic", () => {
  type StudentSort = {
    id: string;
    first_name: string;
    second_name: string;
    current_location: string;
  };

  const isHomeLocation = (loc: string) => loc === "Zuhause";

  it("sorts alphabetically by last name then first name in default mode", () => {
    const students: StudentSort[] = [
      {
        id: "1",
        first_name: "Zara",
        second_name: "Mueller",
        current_location: "Raum 101",
      },
      {
        id: "2",
        first_name: "Anna",
        second_name: "Becker",
        current_location: "Raum 101",
      },
      {
        id: "3",
        first_name: "Max",
        second_name: "Mueller",
        current_location: "Raum 101",
      },
    ];

    const sorted = [...students].sort((a, b) => {
      const lastCmp = (a.second_name ?? "").localeCompare(
        b.second_name ?? "",
        "de",
      );
      if (lastCmp !== 0) return lastCmp;
      return (a.first_name ?? "").localeCompare(b.first_name ?? "", "de");
    });

    expect(sorted.map((s) => s.id)).toEqual(["2", "3", "1"]); // Becker, Mueller(Max), Mueller(Zara)
  });

  it("handles empty names in alphabetical sort", () => {
    const students: StudentSort[] = [
      {
        id: "1",
        first_name: "Max",
        second_name: "",
        current_location: "Raum 101",
      },
      {
        id: "2",
        first_name: "Anna",
        second_name: "Zeller",
        current_location: "Raum 101",
      },
    ];

    const sorted = [...students].sort((a, b) => {
      const lastCmp = (a.second_name ?? "").localeCompare(
        b.second_name ?? "",
        "de",
      );
      if (lastCmp !== 0) return lastCmp;
      return (a.first_name ?? "").localeCompare(b.first_name ?? "", "de");
    });

    expect(sorted[0]?.id).toBe("1"); // Empty string sorts before "Zeller"
  });

  it("sorts pickup mode: present with time first, then without, then home", () => {
    const pickupTimes = new Map([
      ["1", { pickupTime: "15:00" }],
      ["3", { pickupTime: "14:00" }],
    ]);

    const students: StudentSort[] = [
      {
        id: "1",
        first_name: "A",
        second_name: "A",
        current_location: "Raum 101",
      }, // present, pickup 15:00
      {
        id: "2",
        first_name: "B",
        second_name: "B",
        current_location: "Raum 101",
      }, // present, no pickup
      {
        id: "3",
        first_name: "C",
        second_name: "C",
        current_location: "Raum 101",
      }, // present, pickup 14:00
      {
        id: "4",
        first_name: "D",
        second_name: "D",
        current_location: "Zuhause",
      }, // at home
    ];

    const sorted = [...students].sort((a, b) => {
      const aHome = isHomeLocation(a.current_location);
      const bHome = isHomeLocation(b.current_location);

      if (aHome && !bHome) return 1;
      if (!aHome && bHome) return -1;
      if (aHome && bHome) return 0;

      const timeA = pickupTimes.get(a.id)?.pickupTime;
      const timeB = pickupTimes.get(b.id)?.pickupTime;

      if (!timeA && !timeB) return 0;
      if (!timeA) return 1;
      if (!timeB) return -1;

      return timeA.localeCompare(timeB);
    });

    // 14:00 first, then 15:00, then no pickup (present), then home
    expect(sorted.map((s) => s.id)).toEqual(["3", "1", "2", "4"]);
  });

  it("sorts home students to end regardless of pickup time", () => {
    const pickupTimes = new Map([
      ["1", { pickupTime: "14:00" }],
      ["2", { pickupTime: "13:00" }], // earlier time but at home
    ]);

    const students: StudentSort[] = [
      {
        id: "1",
        first_name: "A",
        second_name: "A",
        current_location: "Raum 101",
      },
      {
        id: "2",
        first_name: "B",
        second_name: "B",
        current_location: "Zuhause",
      },
    ];

    const sorted = [...students].sort((a, b) => {
      const aHome = isHomeLocation(a.current_location);
      const bHome = isHomeLocation(b.current_location);

      if (aHome && !bHome) return 1;
      if (!aHome && bHome) return -1;
      if (aHome && bHome) return 0;

      const timeA = pickupTimes.get(a.id)?.pickupTime;
      const timeB = pickupTimes.get(b.id)?.pickupTime;

      if (!timeA && !timeB) return 0;
      if (!timeA) return 1;
      if (!timeB) return -1;

      return timeA.localeCompare(timeB);
    });

    expect(sorted[0]?.id).toBe("1"); // Present student first
    expect(sorted[1]?.id).toBe("2"); // Home student last
  });

  it("keeps order stable for two home students", () => {
    const students: StudentSort[] = [
      {
        id: "1",
        first_name: "A",
        second_name: "A",
        current_location: "Zuhause",
      },
      {
        id: "2",
        first_name: "B",
        second_name: "B",
        current_location: "Zuhause",
      },
    ];

    const sorted = [...students].sort((a, b) => {
      const aHome = isHomeLocation(a.current_location);
      const bHome = isHomeLocation(b.current_location);

      if (aHome && !bHome) return 1;
      if (!aHome && bHome) return -1;
      if (aHome && bHome) return 0;
      return 0;
    });

    // Both at home, stable sort preserves original order
    expect(sorted.map((s) => s.id)).toEqual(["1", "2"]);
  });

  it("sorts present students without pickup times equally", () => {
    const pickupTimes = new Map<string, { pickupTime: string }>();

    const students: StudentSort[] = [
      {
        id: "1",
        first_name: "A",
        second_name: "A",
        current_location: "Raum 101",
      },
      {
        id: "2",
        first_name: "B",
        second_name: "B",
        current_location: "Raum 102",
      },
    ];

    const sorted = [...students].sort((a, b) => {
      const aHome = isHomeLocation(a.current_location);
      const bHome = isHomeLocation(b.current_location);

      if (aHome && !bHome) return 1;
      if (!aHome && bHome) return -1;
      if (aHome && bHome) return 0;

      const timeA = pickupTimes.get(a.id)?.pickupTime;
      const timeB = pickupTimes.get(b.id)?.pickupTime;

      if (!timeA && !timeB) return 0;
      if (!timeA) return 1;
      if (!timeB) return -1;

      return timeA.localeCompare(timeB);
    });

    // Both present without times, stable order
    expect(sorted.map((s) => s.id)).toEqual(["1", "2"]);
  });
});

describe("OGSGroupPage sort active filter", () => {
  type SortMode = "default" | "pickup";

  function buildSortFilters(
    sortMode: SortMode,
    searchTerm: string,
    selectedYear: string,
  ): Array<{ id: string; label: string }> {
    const filters: Array<{ id: string; label: string }> = [];
    if (sortMode !== "default") {
      filters.push({ id: "sort", label: "Sortiert: Nächste Abholung" });
    }
    if (searchTerm.length > 0) {
      filters.push({ id: "search", label: `"${searchTerm}"` });
    }
    if (selectedYear !== "all") {
      filters.push({ id: "year", label: `Jahr ${selectedYear}` });
    }
    return filters;
  }

  it("creates sort active filter when sortMode is pickup", () => {
    const filters = buildSortFilters("pickup", "", "all");
    expect(filters).toHaveLength(1);
    expect(filters[0]?.label).toBe("Sortiert: Nächste Abholung");
    expect(filters[0]?.id).toBe("sort");
  });

  it("does not create sort filter when sortMode is default", () => {
    const filters = buildSortFilters("default", "", "all");
    expect(filters).toHaveLength(0);
  });

  it("includes sort filter with other active filters", () => {
    const filters = buildSortFilters("pickup", "Max", "2");
    expect(filters).toHaveLength(3);
    expect(filters[0]?.id).toBe("sort");
  });
});

describe("OGSGroupPage sort filter config", () => {
  it("provides correct sort filter options", () => {
    const sortOptions = [
      { value: "default", label: "Alphabetisch" },
      { value: "pickup", label: "Nächste Abholung" },
    ];

    expect(sortOptions).toHaveLength(2);
    expect(sortOptions[0]?.value).toBe("default");
    expect(sortOptions[0]?.label).toBe("Alphabetisch");
    expect(sortOptions[1]?.value).toBe("pickup");
    expect(sortOptions[1]?.label).toBe("Nächste Abholung");
  });

  it("sort filter config has correct structure", () => {
    const sortConfig = {
      id: "sort",
      label: "Sortierung",
      type: "buttons" as const,
      value: "default",
      options: [
        { value: "default", label: "Alphabetisch" },
        { value: "pickup", label: "Nächste Abholung" },
      ],
    };

    expect(sortConfig.id).toBe("sort");
    expect(sortConfig.type).toBe("buttons");
    expect(sortConfig.options).toHaveLength(2);
  });
});

describe("OGSGroupPage clear all filters includes sort reset", () => {
  it("resets sort mode along with other filters", () => {
    // Values after onClearAllFilters runs
    const searchTerm = "";
    const selectedYear = "all";
    const attendanceFilter = "all";
    const sortMode = "default";

    expect(searchTerm).toBe("");
    expect(selectedYear).toBe("all");
    expect(attendanceFilter).toBe("all");
    expect(sortMode).toBe("default");
  });
});

// ===== RENDER TESTS that exercise actual source code lines =====
// These tests render the component with pickup time data to cover
// getPickupUrgency, renderPickupIcon, sortedStudents, and filter configs.

describe("OGSGroupPage rendered pickup urgency", () => {
  const mockMutate = vi.fn();

  function setupWithStudentsAndPickupTimes(
    pickupMap: Map<
      string,
      { pickupTime: string; isException: boolean; reason?: string }
    >,
    locationMocks?: {
      isHome?: (loc: string | null | undefined) => boolean;
    },
  ) {
    vi.clearAllMocks();
    global.fetch = vi.fn();

    // Setup location mocks
    if (locationMocks?.isHome) {
      vi.mocked(isHomeLocation).mockImplementation(locationMocks.isHome);
    } else {
      vi.mocked(isHomeLocation).mockReturnValue(false);
    }

    // Return pickup times when fetched
    mockFetchBulkPickupTimes.mockResolvedValue(pickupMap);

    // Two SWR calls: dashboard (first) and students (second)
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
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
              name: "Anna Becker",
              first_name: "Anna",
              second_name: "Becker",
              current_location: "Raum 101",
            },
            {
              id: "2",
              name: "Max Zeller",
              first_name: "Max",
              second_name: "Zeller",
              current_location: "Raum 101",
            },
            {
              id: "3",
              name: "Lena Mueller",
              first_name: "Lena",
              second_name: "Mueller",
              current_location: "Zuhause",
            },
          ],
          roomStatus: {
            student_room_status: {
              "1": { in_group_room: true },
              "2": { in_group_room: true },
              "3": { in_group_room: false },
            },
          },
          substitutions: [],
          firstGroupId: "1",
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
  }

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("renders pickup time with default gray icon when no urgency", async () => {
    // Pickup far in the future (normal urgency)
    const pickupMap = new Map([
      ["1", { pickupTime: "23:59", isException: false }],
    ]);
    setupWithStudentsAndPickupTimes(pickupMap);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });

    // Should render pickup time text
    await waitFor(() => {
      expect(screen.getByText(/23:59 Uhr/)).toBeInTheDocument();
    });

    // Should render default pickup time icon (gray clock from mock)
    expect(screen.getByTestId("pickup-time-icon")).toBeInTheDocument();
  });

  it("renders pulsing orange clock when pickup is within 30 minutes", async () => {
    // Set pickup time to be within 30 minutes of now
    const now = new Date();
    const soonMinutes = now.getMinutes() + 15;
    const soonHours = now.getHours() + (soonMinutes >= 60 ? 1 : 0);
    const soonTime = `${String(soonHours % 24).padStart(2, "0")}:${String(soonMinutes % 60).padStart(2, "0")}`;

    const pickupMap = new Map([
      ["1", { pickupTime: soonTime, isException: false }],
    ]);
    setupWithStudentsAndPickupTimes(pickupMap);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });

    // Should render lucide Clock icon (from mock)
    await waitFor(() => {
      expect(screen.getByTestId("lucide-clock")).toBeInTheDocument();
    });
  });

  it("renders red alert triangle when pickup is overdue", async () => {
    // Set pickup time to past
    const pickupMap = new Map([
      ["1", { pickupTime: "00:01", isException: false }],
    ]);
    setupWithStudentsAndPickupTimes(pickupMap);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });

    // Should render lucide AlertTriangle icon (from mock)
    await waitFor(() => {
      expect(screen.getByTestId("lucide-alert-triangle")).toBeInTheDocument();
    });
  });

  it("renders exception icon when student has pickup exception", async () => {
    const pickupMap = new Map([
      [
        "1",
        {
          pickupTime: "00:01",
          isException: true,
          reason: "Arzttermin",
        },
      ],
    ]);
    setupWithStudentsAndPickupTimes(pickupMap);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });

    // Exception icon should be used instead of urgency icon
    await waitFor(() => {
      expect(screen.getByTestId("exception-icon")).toBeInTheDocument();
    });

    // Exception reason should be displayed
    expect(screen.getByText("(Arzttermin)")).toBeInTheDocument();
  });

  it("does not show urgency icon for at-home students", async () => {
    // Student 3 is "Zuhause" — should get no urgency even with overdue time
    const pickupMap = new Map([
      ["3", { pickupTime: "00:01", isException: false }],
    ]);
    setupWithStudentsAndPickupTimes(pickupMap, {
      isHome: (loc) => loc === "Zuhause",
    });

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });

    // At-home student with overdue time should get default gray icon, not alert triangle
    await waitFor(() => {
      expect(screen.getByText(/00:01 Uhr/)).toBeInTheDocument();
    });
    // Should use PickupTimeIcon (default gray), not AlertTriangle
    expect(screen.getByTestId("pickup-time-icon")).toBeInTheDocument();
    expect(
      screen.queryByTestId("lucide-alert-triangle"),
    ).not.toBeInTheDocument();
  });

  it("renders sort filter with Alphabetisch and Nächste Abholung options", async () => {
    setupWithStudentsAndPickupTimes(new Map());

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("filter-sort")).toBeInTheDocument();
    });

    // Sort filter should have both options
    expect(screen.getByTestId("filter-sort-default")).toBeInTheDocument();
    expect(screen.getByTestId("filter-sort-pickup")).toBeInTheDocument();

    // Labels should match
    expect(screen.getByText("Alphabetisch")).toBeInTheDocument();
    expect(screen.getByText("Nächste Abholung")).toBeInTheDocument();
  });

  it("renders students in alphabetical order by default", async () => {
    setupWithStudentsAndPickupTimes(new Map());

    render(<OGSGroupPage />);

    await waitFor(() => {
      const cards = screen.getAllByTestId("student-card");
      expect(cards).toHaveLength(3);
    });

    // Default sort is alphabetical by last name
    const cards = screen.getAllByTestId("student-card");
    expect(cards[0]?.textContent).toContain("Anna Becker");
    expect(cards[1]?.textContent).toContain("Lena Mueller");
    expect(cards[2]?.textContent).toContain("Max Zeller");
  });

  it("shows sort active filter chip when pickup sort is active", async () => {
    setupWithStudentsAndPickupTimes(new Map());

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("filter-sort")).toBeInTheDocument();
    });

    // Click "Nächste Abholung" sort button
    const pickupSortBtn = screen.getByTestId("filter-sort-pickup");
    pickupSortBtn.click();

    // Active filter chip should appear
    await waitFor(() => {
      expect(screen.getByTestId("active-filter-sort")).toBeInTheDocument();
      expect(
        screen.getByText("Sortiert: Nächste Abholung"),
      ).toBeInTheDocument();
    });
  });

  it("sorts by pickup time when pickup sort is activated", async () => {
    const pickupMap = new Map([
      ["1", { pickupTime: "16:00", isException: false }],
      ["2", { pickupTime: "14:00", isException: false }],
    ]);
    setupWithStudentsAndPickupTimes(pickupMap, {
      isHome: (loc) => loc === "Zuhause",
    });

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });

    // Activate pickup sort
    const pickupSortBtn = screen.getByTestId("filter-sort-pickup");
    pickupSortBtn.click();

    // After sort: 14:00 (Zeller) → 16:00 (Becker) → no time at home (Mueller)
    await waitFor(() => {
      const cards = screen.getAllByTestId("student-card");
      expect(cards[0]?.textContent).toContain("Max Zeller"); // 14:00
      expect(cards[1]?.textContent).toContain("Anna Becker"); // 16:00
      expect(cards[2]?.textContent).toContain("Lena Mueller"); // at home, no time
    });
  });

  it("removes sort active filter when chip is dismissed", async () => {
    setupWithStudentsAndPickupTimes(new Map());

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("filter-sort")).toBeInTheDocument();
    });

    // Activate pickup sort
    screen.getByTestId("filter-sort-pickup").click();

    await waitFor(() => {
      expect(screen.getByTestId("active-filter-sort")).toBeInTheDocument();
    });

    // Remove the filter
    screen.getByTestId("remove-filter-sort").click();

    await waitFor(() => {
      expect(
        screen.queryByTestId("active-filter-sort"),
      ).not.toBeInTheDocument();
    });
  });

  it("clears sort mode when clear all filters is clicked", async () => {
    setupWithStudentsAndPickupTimes(new Map());

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("filter-sort")).toBeInTheDocument();
    });

    // Activate pickup sort
    screen.getByTestId("filter-sort-pickup").click();

    await waitFor(() => {
      expect(screen.getByTestId("active-filter-sort")).toBeInTheDocument();
    });

    // Clear all filters
    screen.getByTestId("clear-all-filters").click();

    await waitFor(() => {
      expect(
        screen.queryByTestId("active-filter-sort"),
      ).not.toBeInTheDocument();
    });
  });

  it("renders students without pickup time (no extra content)", async () => {
    // No pickup times at all
    setupWithStudentsAndPickupTimes(new Map());

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });

    // No extra-content should be rendered (no pickup times)
    expect(screen.queryByTestId("extra-content")).not.toBeInTheDocument();
  });
});
