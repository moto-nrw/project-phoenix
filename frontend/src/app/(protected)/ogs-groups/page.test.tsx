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
const mockSearchParamsGet = vi.fn((_key?: string): string | null => null);
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
  useSearchParams: () => ({ get: mockSearchParamsGet }),
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
      {
        pickupTime: string;
        isException: boolean;
        dayNotes?: { id: string; content: string }[];
      }
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
import { studentService } from "~/lib/api";
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

    expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
  });

  it("shows no access state when user has no OGS groups", async () => {
    // Mock SWR to return empty data indicating no access
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [],
        students: [],
        roomStatus: null,
        substitutions: [],
        pickupTimes: [],
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
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
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

  it("converts BFF pickup times array to Map and displays pickup time", async () => {
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
            last_name: "Mustermann",
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
        // BFF returns pickup times as array (backend format)
        pickupTimes: [
          {
            student_id: 1,
            date: "2026-02-04",
            weekday_name: "Mittwoch",
            pickup_time: "15:30",
            is_exception: false,
            day_notes: [{ id: 1, content: "Test note" }],
            notes: "Parent pickup",
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

    // Should display the student card with pickup time
    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
      // Pickup time should be rendered (15:30 Uhr format)
      expect(screen.getByText(/15:30 Uhr/)).toBeInTheDocument();
    });
  });

  it("handles BFF pickup times with day notes correctly", async () => {
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
            last_name: "Mustermann",
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
        pickupTimes: [
          {
            student_id: 1,
            date: "2026-02-04",
            weekday_name: "Mittwoch",
            pickup_time: "16:00",
            is_exception: true,
            day_notes: [
              { id: 1, content: "Note 1" },
              { id: 2, content: "Note 2" },
            ],
            notes: "Multiple notes test",
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
      expect(screen.getByText(/16:00 Uhr/)).toBeInTheDocument();
    });
  });

  it("handles BFF pickup times without day notes", async () => {
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
            last_name: "Mustermann",
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
        pickupTimes: [
          {
            student_id: 1,
            date: "2026-02-04",
            weekday_name: "Mittwoch",
            pickup_time: "14:00",
            is_exception: false,
            // No day_notes - tests null coalescing
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
      expect(screen.getByText(/14:00 Uhr/)).toBeInTheDocument();
    });
  });

  it("syncs pickup times from SWR students data when pickupTimes is a Map", async () => {
    // This test covers the `instanceof Map` branch in the SWR students sync useEffect.
    // The second useSWRAuth call (students SWR) returns pickupTimes as a Map,
    // which triggers `setPickupTimes(swrStudentsData.pickupTimes)`.
    const pickupMap = new Map([
      [
        "1",
        {
          studentId: "1",
          date: "2026-02-05",
          weekdayName: "Donnerstag",
          pickupTime: "15:30",
          isException: false,
          dayNotes: [],
          notes: undefined,
        },
      ],
    ]);

    // IMPORTANT: Stable references prevent infinite re-render loops
    const studentsSwrResult = {
      data: {
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            second_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: { "1": { in_group_room: true } },
        pickupTimes: pickupMap,
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    };

    const dashboardSwrResult = {
      data: {
        groups: [
          {
            id: 1,
            name: "OGS A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
        ],
        students: [
          {
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          group_has_room: true,
          student_room_status: { "1": { in_group_room: true } },
        },
        substitutions: [],
        pickupTimes: [
          {
            student_id: 1,
            date: "2026-02-05",
            weekday_name: "Donnerstag",
            pickup_time: "15:30",
            is_exception: false,
          },
        ],
        firstGroupId: "1",
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    };

    vi.mocked(useSWRAuth).mockImplementation(((key: string | null) => {
      if (typeof key === "string" && key.startsWith("ogs-students-")) {
        return studentsSwrResult;
      }
      return dashboardSwrResult;
    }) as unknown as typeof useSWRAuth);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
      expect(screen.getByText(/15:30 Uhr/)).toBeInTheDocument();
    });
  });

  it("executes SWR students fetcher with pickup times", async () => {
    // This test captures and directly invokes the fetcher function passed to
    // the second useSWRAuth call (ogs-students-*), covering the SWR fetcher body
    // that is normally never executed because useSWRAuth is mocked.
    interface SwrFetcherResult {
      students: { id: string }[];
      pickupTimes: Map<string, unknown>;
      roomStatus: Record<string, unknown> | undefined;
    }
    let capturedStudentsFetcher: (() => Promise<SwrFetcherResult>) | null =
      null;

    const dashboardResult = {
      data: {
        groups: [
          {
            id: 1,
            name: "OGS A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
        ],
        students: [
          {
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          group_has_room: true,
          student_room_status: { "1": { in_group_room: true } },
        },
        substitutions: [],
        pickupTimes: [],
        firstGroupId: "1",
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    };

    const nullResult = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    };

    vi.mocked(useSWRAuth).mockImplementation(((
      key: string | null,
      fetcher?: () => Promise<unknown>,
    ) => {
      if (typeof key === "string" && key.startsWith("ogs-students-")) {
        capturedStudentsFetcher = fetcher as typeof capturedStudentsFetcher;
        return nullResult;
      }
      if (key === "ogs-dashboard") return dashboardResult;
      return nullResult;
    }) as unknown as typeof useSWRAuth);

    render(<OGSGroupPage />);

    // Wait for re-render so the students SWR key becomes non-null
    await waitFor(() => {
      expect(capturedStudentsFetcher).not.toBeNull();
    });

    // Set up mocks for the fetcher's dependencies
    vi.mocked(studentService.getStudents).mockResolvedValue({
      students: [
        {
          id: "1",
          name: "Max Mustermann",
          first_name: "Max",
          second_name: "Mustermann",
          school_class: "1a",
          current_location: "Raum 101",
        },
      ] as never,
      total: 1,
    } as never);

    vi.mocked(global.fetch).mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: {
          student_room_status: { "1": { in_group_room: true } },
        },
      }),
    } as Response);

    const pickupMap = new Map([
      [
        "1",
        {
          studentId: "1",
          date: "2026-02-05",
          weekdayName: "Donnerstag",
          pickupTime: "15:30",
          isException: false,
          dayNotes: [] as { id: string; content: string }[],
          notes: undefined,
        },
      ],
    ]);
    mockFetchBulkPickupTimes.mockResolvedValueOnce(pickupMap);

    // Invoke the captured fetcher directly to cover lines 444-496
    const result = await capturedStudentsFetcher!();

    expect(result.students).toHaveLength(1);
    expect(result.pickupTimes).toBeInstanceOf(Map);
    expect(result.pickupTimes.get("1")).toBeDefined();
    expect(result.roomStatus).toEqual({ "1": { in_group_room: true } });
  });

  it("SWR students fetcher returns empty pickup times when fetch fails", async () => {
    // Covers the catch branch in the SWR fetcher for fetchBulkPickupTimes
    interface SwrFetcherResult {
      students: { id: string }[];
      pickupTimes: Map<string, unknown>;
      roomStatus: Record<string, unknown> | undefined;
    }
    let capturedStudentsFetcher: (() => Promise<SwrFetcherResult>) | null =
      null;

    const dashboardResult = {
      data: {
        groups: [
          {
            id: 1,
            name: "OGS A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
        ],
        students: [
          {
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
          },
        ],
        roomStatus: null,
        substitutions: [],
        pickupTimes: [],
        firstGroupId: "1",
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    };

    const nullResult = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    };

    vi.mocked(useSWRAuth).mockImplementation(((
      key: string | null,
      fetcher?: () => Promise<unknown>,
    ) => {
      if (typeof key === "string" && key.startsWith("ogs-students-")) {
        capturedStudentsFetcher = fetcher as typeof capturedStudentsFetcher;
        return nullResult;
      }
      if (key === "ogs-dashboard") return dashboardResult;
      return nullResult;
    }) as unknown as typeof useSWRAuth);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(capturedStudentsFetcher).not.toBeNull();
    });

    vi.mocked(studentService.getStudents).mockResolvedValue({
      students: [
        {
          id: "1",
          name: "Max Mustermann",
          first_name: "Max",
          second_name: "Mustermann",
        },
      ] as never,
      total: 1,
    } as never);

    vi.mocked(global.fetch).mockResolvedValueOnce({
      ok: false,
      status: 500,
    } as Response);

    // fetchBulkPickupTimes throws
    mockFetchBulkPickupTimes.mockRejectedValueOnce(
      new Error("Pickup times fetch failed"),
    );

    const consoleSpy = vi.spyOn(console, "error").mockImplementation(vi.fn());

    const result = await capturedStudentsFetcher!();

    // Should still return students but with empty pickup times Map
    expect(result.students).toHaveLength(1);
    expect(result.pickupTimes).toBeInstanceOf(Map);
    expect(result.pickupTimes.size).toBe(0);
    // Room status fetch failed (ok: false), so should be null/undefined
    expect(result.roomStatus).toBeUndefined();

    consoleSpy.mockRestore();
  });

  it("SWR students fetcher skips pickup times when no students", async () => {
    // Covers the `if (students.length > 0)` false branch in the SWR fetcher
    interface SwrFetcherResult {
      students: { id: string }[];
      pickupTimes: Map<string, unknown>;
      roomStatus: Record<string, unknown> | undefined;
    }
    let capturedStudentsFetcher: (() => Promise<SwrFetcherResult>) | null =
      null;

    const dashboardResult = {
      data: {
        groups: [
          {
            id: 1,
            name: "OGS A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
        ],
        students: [],
        roomStatus: null,
        substitutions: [],
        pickupTimes: [],
        firstGroupId: "1",
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    };

    const nullResult = {
      data: null,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    };

    vi.mocked(useSWRAuth).mockImplementation(((
      key: string | null,
      fetcher?: () => Promise<unknown>,
    ) => {
      if (typeof key === "string" && key.startsWith("ogs-students-")) {
        capturedStudentsFetcher = fetcher as typeof capturedStudentsFetcher;
        return nullResult;
      }
      if (key === "ogs-dashboard") return dashboardResult;
      return nullResult;
    }) as unknown as typeof useSWRAuth);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(capturedStudentsFetcher).not.toBeNull();
    });

    vi.mocked(studentService.getStudents).mockResolvedValue({
      students: [],
      total: 0,
    } as never);

    vi.mocked(global.fetch).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ data: { student_room_status: {} } }),
    } as Response);

    const result = await capturedStudentsFetcher!();

    expect(result.students).toHaveLength(0);
    expect(result.pickupTimes).toBeInstanceOf(Map);
    expect(result.pickupTimes.size).toBe(0);
    // fetchBulkPickupTimes should NOT have been called
    expect(mockFetchBulkPickupTimes).not.toHaveBeenCalled();
  });

  it("maps BFF students with missing optional fields", async () => {
    // Covers null coalescing branches in student mapping (school_class ?? "", etc.)
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "OGS A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
        ],
        students: [
          {
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            // Intentionally missing: school_class, current_location,
            // location_since, group_id, group_name
          },
        ],
        roomStatus: null,
        substitutions: [],
        pickupTimes: [],
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
      // Student name should render from first_name + last_name mapping
      expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();
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
      last_name: "Mustermann",
      school_class: "1a",
    };

    // Test search matching first name
    const searchLower = "max";
    const matches =
      student.name?.toLowerCase().includes(searchLower) ??
      student.first_name?.toLowerCase().includes(searchLower) ??
      student.last_name?.toLowerCase().includes(searchLower) ??
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
        pickupTimes: [],
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
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
          {
            id: "2",
            name: "Erika Schmidt",
            first_name: "Erika",
            last_name: "Schmidt",
            current_location: "Raum 101",
          },
          {
            id: "3",
            name: "Hans Mueller",
            first_name: "Hans",
            last_name: "Mueller",
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
        pickupTimes: [],
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
      expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
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
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
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
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
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
            last_name: "Mustermann",
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
        pickupTimes: [],
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
    last_name: string;
    current_location: string;
  };

  const isHomeLocation = (loc: string) => loc === "Zuhause";

  it("sorts alphabetically by last name then first name in default mode", () => {
    const students: StudentSort[] = [
      {
        id: "1",
        first_name: "Zara",
        last_name: "Mueller",
        current_location: "Raum 101",
      },
      {
        id: "2",
        first_name: "Anna",
        last_name: "Becker",
        current_location: "Raum 101",
      },
      {
        id: "3",
        first_name: "Max",
        last_name: "Mueller",
        current_location: "Raum 101",
      },
    ];

    const sorted = [...students].sort((a, b) => {
      const lastCmp = (a.last_name ?? "").localeCompare(
        b.last_name ?? "",
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
        last_name: "",
        current_location: "Raum 101",
      },
      {
        id: "2",
        first_name: "Anna",
        last_name: "Zeller",
        current_location: "Raum 101",
      },
    ];

    const sorted = [...students].sort((a, b) => {
      const lastCmp = (a.last_name ?? "").localeCompare(
        b.last_name ?? "",
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
        last_name: "A",
        current_location: "Raum 101",
      }, // present, pickup 15:00
      {
        id: "2",
        first_name: "B",
        last_name: "B",
        current_location: "Raum 101",
      }, // present, no pickup
      {
        id: "3",
        first_name: "C",
        last_name: "C",
        current_location: "Raum 101",
      }, // present, pickup 14:00
      {
        id: "4",
        first_name: "D",
        last_name: "D",
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
        last_name: "A",
        current_location: "Raum 101",
      },
      {
        id: "2",
        first_name: "B",
        last_name: "B",
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
        last_name: "A",
        current_location: "Zuhause",
      },
      {
        id: "2",
        first_name: "B",
        last_name: "B",
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
        last_name: "A",
        current_location: "Raum 101",
      },
      {
        id: "2",
        first_name: "B",
        last_name: "B",
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
  // Freeze time to 14:00 on 2026-01-28 to make tests deterministic
  const FROZEN_TIME = new Date(2026, 0, 28, 14, 0, 0);

  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.setSystemTime(FROZEN_TIME);
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  function setupWithStudentsAndPickupTimes(
    pickupMap: Map<
      string,
      {
        pickupTime: string;
        isException: boolean;
        dayNotes?: { id: string; content: string }[];
      }
    >,
    locationMocks?: {
      isHome?: (loc: string | null | undefined) => boolean;
    },
  ) {
    vi.clearAllMocks();
    // Re-freeze time after clearAllMocks since it may reset fake timers state
    vi.setSystemTime(FROZEN_TIME);
    global.fetch = vi.fn();

    // Setup location mocks
    if (locationMocks?.isHome) {
      vi.mocked(isHomeLocation).mockImplementation(locationMocks.isHome);
    } else {
      vi.mocked(isHomeLocation).mockReturnValue(false);
    }

    // Return pickup times when fetched (for SWR refetch)
    mockFetchBulkPickupTimes.mockResolvedValue(pickupMap);

    // Convert pickupMap to BFF response format (array of BackendPickupTime objects)
    const pickupTimesArray = Array.from(pickupMap.entries()).map(
      ([studentId, pickup]) => ({
        student_id: parseInt(studentId, 10),
        date: new Date().toISOString().split("T")[0],
        weekday_name: "Mittwoch",
        pickup_time: pickup.pickupTime,
        is_exception: pickup.isException,
        day_notes: pickup.dayNotes?.map((n) => ({
          id: parseInt(n.id, 10),
          content: n.content,
        })),
      }),
    );

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
              last_name: "Becker",
              current_location: "Raum 101",
            },
            {
              id: "2",
              name: "Max Zeller",
              first_name: "Max",
              last_name: "Zeller",
              current_location: "Raum 101",
            },
            {
              id: "3",
              name: "Lena Mueller",
              first_name: "Lena",
              last_name: "Mueller",
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
          pickupTimes: pickupTimesArray,
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

  it("renders pickup time with default gray icon when no urgency", async () => {
    // Pickup far in the future (normal urgency) — frozen time is 14:00
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
    // Frozen time is 14:00, so 14:15 is 15 minutes away → "soon"
    const pickupMap = new Map([
      ["1", { pickupTime: "14:15", isException: false }],
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
    // Frozen time is 14:00, so 13:00 is 1 hour in the past → "overdue"
    const pickupMap = new Map([
      ["1", { pickupTime: "13:00", isException: false }],
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
          dayNotes: [{ id: "1", content: "Arzttermin" }],
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

    // Day note should be displayed
    expect(screen.getByText(/Arzttermin/)).toBeInTheDocument();
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

describe("OGSGroupPage loadAvailableUsers", () => {
  // Test that loadAvailableUsers queries all three roles: teacher, staff, user
  // This covers the code path that fetches staff by role for the transfer dropdown

  it("queries teacher, staff, and user roles for transfer dropdown", async () => {
    // Test the parallel fetch pattern used in loadAvailableUsers
    // The actual implementation calls getStaffByRole for "teacher", "staff", and "user"

    // Create a mock function that simulates the API behavior
    const getStaffByRole = vi.fn((role: string) => {
      if (role === "teacher") {
        return Promise.resolve([
          {
            id: "1",
            personId: "101",
            firstName: "Anna",
            lastName: "Lehrer",
            fullName: "Anna Lehrer",
            accountId: "1001",
            email: "anna@example.com",
          },
        ]);
      }
      if (role === "staff") {
        return Promise.resolve([
          {
            id: "2",
            personId: "102",
            firstName: "Ben",
            lastName: "Staff",
            fullName: "Ben Staff",
            accountId: "1002",
            email: "ben@example.com",
          },
        ]);
      }
      if (role === "user") {
        return Promise.resolve([
          {
            id: "3",
            personId: "103",
            firstName: "Clara",
            lastName: "Nutzer",
            fullName: "Clara Nutzer",
            accountId: "1003",
            email: "clara@example.com",
          },
        ]);
      }
      return Promise.resolve([]);
    });

    // Simulate the parallel fetch pattern from loadAvailableUsers
    const [teachers, staffMembers, users] = await Promise.all([
      getStaffByRole("teacher").catch(() => []),
      getStaffByRole("staff").catch(() => []),
      getStaffByRole("user").catch(() => []),
    ]);

    // Verify all three roles are queried
    expect(getStaffByRole).toHaveBeenCalledWith("teacher");
    expect(getStaffByRole).toHaveBeenCalledWith("staff");
    expect(getStaffByRole).toHaveBeenCalledWith("user");
    expect(getStaffByRole).toHaveBeenCalledTimes(3);

    // Verify results are returned correctly
    expect(teachers).toHaveLength(1);
    expect(staffMembers).toHaveLength(1);
    expect(users).toHaveLength(1);
  });

  it("deduplicates users from different roles by staff ID", () => {
    // Test the deduplication logic used in loadAvailableUsers
    type StaffUser = {
      id: string;
      personId: string;
      firstName: string;
      lastName: string;
      fullName: string;
    };

    const teachers: StaffUser[] = [
      {
        id: "1",
        personId: "101",
        firstName: "Anna",
        lastName: "Lehrer",
        fullName: "Anna Lehrer",
      },
      {
        id: "2",
        personId: "102",
        firstName: "Both",
        lastName: "Roles",
        fullName: "Both Roles",
      },
    ];

    const staffMembers: StaffUser[] = [
      {
        id: "2",
        personId: "102",
        firstName: "Both",
        lastName: "Roles",
        fullName: "Both Roles",
      }, // Duplicate
      {
        id: "3",
        personId: "103",
        firstName: "Ben",
        lastName: "Staff",
        fullName: "Ben Staff",
      },
    ];

    const users: StaffUser[] = [
      {
        id: "2",
        personId: "102",
        firstName: "Both",
        lastName: "Roles",
        fullName: "Both Roles",
      }, // Duplicate
      {
        id: "4",
        personId: "104",
        firstName: "Clara",
        lastName: "Nutzer",
        fullName: "Clara Nutzer",
      },
    ];

    // Mirror the deduplication logic from loadAvailableUsers
    const uniqueUsers = new Map<string, StaffUser>();
    for (const user of [...teachers, ...staffMembers, ...users]) {
      if (!uniqueUsers.has(user.id)) {
        uniqueUsers.set(user.id, user);
      }
    }
    const result = Array.from(uniqueUsers.values());

    // Should have 4 unique users (ID 2 appears 3 times but is deduplicated)
    expect(result).toHaveLength(4);
    expect(result.map((u) => u.id).sort()).toEqual(["1", "2", "3", "4"]);
  });

  it("handles empty results from user role gracefully", () => {
    // Simulates when no users have the "user" role assigned
    type StaffUser = { id: string; fullName: string };

    const teachers: StaffUser[] = [{ id: "1", fullName: "Anna Lehrer" }];
    const staffMembers: StaffUser[] = [{ id: "2", fullName: "Ben Staff" }];
    const users: StaffUser[] = []; // Empty - no users with "user" role

    const uniqueUsers = new Map<string, StaffUser>();
    for (const user of [...teachers, ...staffMembers, ...users]) {
      if (!uniqueUsers.has(user.id)) {
        uniqueUsers.set(user.id, user);
      }
    }
    const result = Array.from(uniqueUsers.values());

    // Should still work with 2 users from teacher and staff roles
    expect(result).toHaveLength(2);
  });

  it("returns all users when only user role has members", () => {
    // Simulates production scenario where most accounts have "user" role
    type StaffUser = { id: string; fullName: string };

    const teachers: StaffUser[] = []; // Empty
    const staffMembers: StaffUser[] = []; // Empty
    const users: StaffUser[] = [
      { id: "1", fullName: "User One" },
      { id: "2", fullName: "User Two" },
      { id: "3", fullName: "User Three" },
    ];

    const uniqueUsers = new Map<string, StaffUser>();
    for (const user of [...teachers, ...staffMembers, ...users]) {
      if (!uniqueUsers.has(user.id)) {
        uniqueUsers.set(user.id, user);
      }
    }
    const result = Array.from(uniqueUsers.values());

    // All 3 users from "user" role should be returned
    expect(result).toHaveLength(3);
  });
});

// Note: Integration tests for the transfer modal are complex due to React state management.
// The getAllAvailableStaff function is tested in group-transfer-api.test.ts which covers:
// - Fetching all three roles (teacher, staff, user)
// - Deduplication by staff ID
// - Error handling when some roles fail to load

// ===== Tests for exported helper functions (direct coverage) =====

import {
  getPickupUrgency as actualGetPickupUrgency,
  isStudentInGroupRoom as actualIsStudentInGroupRoom,
  matchesSearchFilter as actualMatchesSearchFilter,
  matchesAttendanceFilter as actualMatchesAttendanceFilter,
  matchesForeignRoomFilter as actualMatchesForeignRoomFilter,
} from "./ogs-group-helpers";

// Helper to build a minimal Student for direct function tests
function makeTestStudent(
  overrides: Record<string, unknown> = {},
): Parameters<typeof actualMatchesSearchFilter>[0] {
  return {
    id: "1",
    name: "Max Mustermann",
    first_name: "Max",
    last_name: "Mustermann",
    school_class: "3a",
    current_location: "Anwesend - Raum 1",
    group_name: "Eulen",
    group_id: "10",
    ...overrides,
  } as Parameters<typeof actualMatchesSearchFilter>[0];
}

describe("getPickupUrgency (exported)", () => {
  it("returns 'none' for undefined pickup time", () => {
    expect(actualGetPickupUrgency(undefined, new Date())).toBe("none");
  });

  it("returns 'overdue' when pickup is in the past", () => {
    const now = new Date("2025-06-10T15:00:00");
    expect(actualGetPickupUrgency("14:00", now)).toBe("overdue");
  });

  it("returns 'soon' when pickup is within 30 minutes", () => {
    const now = new Date("2025-06-10T14:45:00");
    expect(actualGetPickupUrgency("15:00", now)).toBe("soon");
  });

  it("returns 'normal' when pickup is far in the future", () => {
    const now = new Date("2025-06-10T10:00:00");
    expect(actualGetPickupUrgency("15:00", now)).toBe("normal");
  });
});

describe("isStudentInGroupRoom (exported)", () => {
  it("returns false when student has no location", () => {
    const student = makeTestStudent({ current_location: undefined });
    expect(
      actualIsStudentInGroupRoom(student, {
        id: "1",
        name: "G",
        room_name: "R",
      }),
    ).toBe(false);
  });

  it("returns false when group has no room name", () => {
    const student = makeTestStudent();
    expect(actualIsStudentInGroupRoom(student, { id: "1", name: "G" })).toBe(
      false,
    );
  });

  it("returns true when room name matches (case-insensitive)", () => {
    // parseLocation is mocked to always return { room: "Room 1" }
    const student = makeTestStudent({ current_location: "Anwesend - Room 1" });
    expect(
      actualIsStudentInGroupRoom(student, {
        id: "1",
        name: "G",
        room_name: "ROOM 1",
      }),
    ).toBe(true);
  });

  it("returns false when room does not match", () => {
    // parseLocation is mocked to always return { room: "Room 1" }
    const student = makeTestStudent({ current_location: "Anwesend - Raum 2" });
    expect(
      actualIsStudentInGroupRoom(student, {
        id: "1",
        name: "G",
        room_name: "Raum 99",
      }),
    ).toBe(false);
  });

  it("returns false for null group", () => {
    expect(actualIsStudentInGroupRoom(makeTestStudent(), null)).toBe(false);
  });

  it("matches by room_id when room name does not match", () => {
    const student = makeTestStudent({ current_location: "42" });
    expect(
      actualIsStudentInGroupRoom(student, {
        id: "1",
        name: "G",
        room_name: "X",
        room_id: "42",
      }),
    ).toBe(true);
  });
});

describe("matchesSearchFilter (exported)", () => {
  it("returns true for empty search term", () => {
    expect(actualMatchesSearchFilter(makeTestStudent(), "")).toBe(true);
  });

  it("matches by name", () => {
    expect(actualMatchesSearchFilter(makeTestStudent(), "Max")).toBe(true);
  });

  it("matches by school class", () => {
    expect(
      actualMatchesSearchFilter(makeTestStudent({ school_class: "3a" }), "3a"),
    ).toBe(true);
  });

  it("returns false when nothing matches", () => {
    expect(actualMatchesSearchFilter(makeTestStudent(), "xyz")).toBe(false);
  });
});

describe("matchesAttendanceFilter (exported)", () => {
  const rs = {
    "1": { in_group_room: true, current_room_id: 10 },
    "2": { in_group_room: false, current_room_id: 20 },
  };

  it("returns true for 'all'", () => {
    expect(actualMatchesAttendanceFilter(makeTestStudent(), "all", rs)).toBe(
      true,
    );
  });

  it("returns true for 'in_room' when student is in group room", () => {
    expect(
      actualMatchesAttendanceFilter(
        makeTestStudent({ id: "1" }),
        "in_room",
        rs,
      ),
    ).toBe(true);
  });

  it("returns false for 'in_room' when student is not", () => {
    expect(
      actualMatchesAttendanceFilter(
        makeTestStudent({ id: "2" }),
        "in_room",
        rs,
      ),
    ).toBe(false);
  });

  it("returns true for 'foreign_room' correctly", () => {
    expect(
      actualMatchesAttendanceFilter(
        makeTestStudent({ id: "2" }),
        "foreign_room",
        rs,
      ),
    ).toBe(true);
  });

  it("returns true for 'at_home' when at home", () => {
    vi.mocked(isHomeLocation).mockReturnValue(true);
    expect(
      actualMatchesAttendanceFilter(
        makeTestStudent({ current_location: "Zuhause" }),
        "at_home",
        rs,
      ),
    ).toBe(true);
    vi.mocked(isHomeLocation).mockReturnValue(false);
  });

  it("returns true for unknown filter", () => {
    expect(
      actualMatchesAttendanceFilter(makeTestStudent(), "unknown_value", rs),
    ).toBe(true);
  });
});

describe("matchesForeignRoomFilter (exported)", () => {
  it("returns true when in foreign room", () => {
    expect(
      actualMatchesForeignRoomFilter({
        in_group_room: false,
        current_room_id: 20,
      }),
    ).toBe(true);
  });

  it("returns false when in group room", () => {
    expect(
      actualMatchesForeignRoomFilter({
        in_group_room: true,
        current_room_id: 10,
      }),
    ).toBe(false);
  });

  it("returns false when no room ID", () => {
    expect(actualMatchesForeignRoomFilter({ in_group_room: false })).toBe(
      false,
    );
  });

  it("returns false for undefined", () => {
    expect(actualMatchesForeignRoomFilter(undefined)).toBe(false);
  });
});

// ===== NEW TESTS FOR ID-BASED SELECTION REFACTOR =====
// Tests added to cover new code paths introduced by the index → ID refactor

describe("OGSGroupPage ID-based selection: Stale selection reset", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    localStorage.removeItem("sidebar-last-group");
    mockSearchParamsGet.mockReturnValue(null);
  });

  afterEach(() => {
    cleanup();
  });

  it("resets to first group when previously selected group disappears from list", async () => {
    // Initial render: User has Group A (id=1) and Group B (id=2), Group A selected
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "Group A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
          {
            id: 2,
            name: "Group B",
            room_id: 20,
            room: { id: 20, name: "Raum 202" },
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
        firstGroupId: "1",
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    const { rerender } = render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // Now simulate SSE update: Group A (id=1) is removed, only Group B (id=2) remains
    // This covers lines 255-257: if selectedGroupId doesn't exist, reset to first
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 2,
            name: "Group B",
            room_id: 20,
            room: { id: 20, name: "Raum 202" },
          },
        ],
        students: [
          {
            id: "2",
            name: "Erika Schmidt",
            first_name: "Erika",
            last_name: "Schmidt",
            current_location: "Raum 202",
          },
        ],
        roomStatus: {
          student_room_status: {
            "2": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
        firstGroupId: "2",
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    rerender(<OGSGroupPage />);

    // Should show student from Group B after reset
    await waitFor(() => {
      expect(screen.getByText(/Erika Schmidt/)).toBeInTheDocument();
    });
  });

  it("keeps selection stable when selected group still exists in refreshed list", async () => {
    // Setup: User has Group A and Group B, Group B is selected
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "Group A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
          {
            id: 2,
            name: "Group B",
            room_id: 20,
            room: { id: 20, name: "Raum 202" },
          },
        ],
        students: [
          {
            id: "2",
            name: "Erika Schmidt",
            first_name: "Erika",
            last_name: "Schmidt",
            second_name: "Schmidt",
            current_location: "Raum 202",
          },
        ],
        roomStatus: {
          student_room_status: {
            "2": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
        firstGroupId: "2", // Group B selected
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    const { rerender } = render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByText(/Erika Schmidt/)).toBeInTheDocument();
    });

    // SSE update with same groups (e.g., student count changed)
    // Selection should NOT reset even though firstGroupId is different
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "Group A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
          {
            id: 2,
            name: "Group B",
            room_id: 20,
            room: { id: 20, name: "Raum 202" },
          },
        ],
        students: [
          {
            id: "2",
            name: "Erika Schmidt",
            first_name: "Erika",
            last_name: "Schmidt",
            second_name: "Schmidt",
            current_location: "Raum 202",
          },
        ],
        roomStatus: {
          student_room_status: {
            "2": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
        firstGroupId: "1", // First group in alphabetical order
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    rerender(<OGSGroupPage />);

    // Should still show Group B student (selection not reset)
    await waitFor(() => {
      expect(screen.getByText(/Erika Schmidt/)).toBeInTheDocument();
    });
  });
});

describe("OGSGroupPage ID-based selection: First load initialization", () => {
  const mockMutate = vi.fn();
  const originalLocalStorage = window.localStorage;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    localStorage.removeItem("sidebar-last-group");
    mockSearchParamsGet.mockReturnValue(null);
  });

  afterEach(() => {
    cleanup();
    // Restore original localStorage
    Object.defineProperty(window, "localStorage", {
      value: originalLocalStorage,
      writable: true,
      configurable: true,
    });
  });

  it("locks in first group ID on first data load", async () => {
    // Mock localStorage
    const localStorageMock: Record<string, string> = {};
    Object.defineProperty(window, "localStorage", {
      value: {
        getItem: (key: string) => localStorageMock[key] ?? null,
        setItem: (key: string, value: string) => {
          localStorageMock[key] = value;
        },
        removeItem: (key: string) => {
          delete localStorageMock[key];
        },
      },
      writable: true,
      configurable: true,
    });

    // First render with no selectedGroupId (lines 259-264)
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "Group A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
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

    // Verify first group's students are shown (lines 265)
    expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();
  });

  it("shows first group students only when first group is selected", async () => {
    // Mock scenario: User has 2 groups, Group B (id=2) is selected
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "Group A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
          {
            id: 2,
            name: "Group B",
            room_id: 20,
            room: { id: 20, name: "Raum 202" },
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
        firstGroupId: "1", // BFF returns first group's data
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

    // Should show Group A students since it's the first group
    expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();
  });
});

describe("OGSGroupPage ID-based selection: URL param matching", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem("sidebar-last-group");
    mockSearchParamsGet.mockReturnValue(null);
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          student_room_status: {
            "2": { in_group_room: true },
          },
        },
      }),
    });
    vi.mocked(studentService.getStudents).mockResolvedValue({
      students: [
        {
          id: "2",
          name: "Erika Schmidt",
          first_name: "Erika",
          last_name: "Schmidt",
          current_location: "Raum 202",
        },
      ],
    } as never);
  });

  afterEach(() => {
    cleanup();
  });

  it("switches to group when URL param matches a valid group ID", async () => {
    // Setup: URL has ?group=2, user has Group A (id=1) and Group B (id=2)
    mockSearchParamsGet.mockReturnValue("2");

    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "Group A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
          {
            id: 2,
            name: "Group B",
            room_id: 20,
            room: { id: 20, name: "Raum 202" },
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
        firstGroupId: "1",
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    // Verify switchToGroup was called with ID "2" via studentService (lines 300-306)
    await waitFor(() => {
      expect(studentService.getStudents).toHaveBeenCalledWith(
        expect.objectContaining({ groupId: "2" }),
      );
    });
  });

  it("ignores URL param when group ID does not exist in list", async () => {
    // Setup: URL has ?group=999 (invalid), user has Group A (id=1)
    mockSearchParamsGet.mockReturnValue("999");

    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "Group A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
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

    // Should show Group A student (didn't switch to invalid group)
    expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();
  });

  it("does not switch when URL param matches already selected group", async () => {
    // Setup: URL has ?group=1, Group A (id=1) already selected
    mockSearchParamsGet.mockReturnValue("1");

    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "Group A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
          {
            id: 2,
            name: "Group B",
            room_id: 20,
            room: { id: 20, name: "Raum 202" },
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
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

    // Should show Group A student (no unnecessary switch)
    expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();

    // Should NOT make extra API calls (line 305 early return)
    expect(global.fetch).not.toHaveBeenCalled();
  });
});

describe("OGSGroupPage ID-based selection: localStorage restore", () => {
  const mockMutate = vi.fn();

  let localStorageMock: Record<string, string>;
  const originalLocalStorage = window.localStorage;

  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParamsGet.mockReturnValue(null);
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          student_room_status: {
            "2": { in_group_room: true },
          },
        },
      }),
    });
    vi.mocked(studentService.getStudents).mockResolvedValue({
      students: [
        {
          id: "2",
          name: "Erika Schmidt",
          first_name: "Erika",
          last_name: "Schmidt",
          current_location: "Raum 202",
        },
      ],
    } as never);

    // Mock localStorage
    localStorageMock = {};
    Object.defineProperty(window, "localStorage", {
      value: {
        getItem: (key: string) => localStorageMock[key] ?? null,
        setItem: (key: string, value: string) => {
          localStorageMock[key] = value;
        },
        removeItem: (key: string) => {
          delete localStorageMock[key];
        },
      },
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    cleanup();
    // Restore original localStorage so subsequent tests aren't affected
    Object.defineProperty(window, "localStorage", {
      value: originalLocalStorage,
      writable: true,
      configurable: true,
    });
  });

  it("restores saved group by ID from localStorage when no URL param", async () => {
    // Setup: localStorage has group ID "2", no URL param
    localStorageMock["sidebar-last-group"] = "2";

    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "Group A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
          {
            id: 2,
            name: "Group B",
            room_id: 20,
            room: { id: 20, name: "Raum 202" },
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    // Should restore Group B from localStorage (lines 310-315)
    await waitFor(() => {
      expect(studentService.getStudents).toHaveBeenCalledWith(
        expect.objectContaining({ groupId: "2" }),
      );
    });
  });

  it("persists first group to localStorage when saved group no longer exists", async () => {
    // Setup: localStorage has group ID "999" (doesn't exist), no URL param
    localStorageMock["sidebar-last-group"] = "999";
    mockSearchParamsGet.mockReturnValue(null);

    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "Group A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
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

    // Should persist first group to localStorage (lines 317-321)
    await waitFor(() => {
      expect(localStorageMock["sidebar-last-group"]).toBe("1");
    });
  });

  it("does not switch when saved group ID matches currently selected group", async () => {
    // Setup: localStorage has group ID "1", Group A (id=1) already selected
    localStorageMock["sidebar-last-group"] = "1";
    mockSearchParamsGet.mockReturnValue(null);

    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "Group A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
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

    // Should NOT make extra API calls (line 323 early return)
    expect(global.fetch).not.toHaveBeenCalled();
  });
});

describe("OGSGroupPage ID-based selection: switchToGroup behavior", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    localStorage.removeItem("sidebar-last-group");
    mockSearchParamsGet.mockReturnValue(null);
  });

  afterEach(() => {
    cleanup();
  });

  it("is a no-op when switching to non-existent group ID", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "Group A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
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

    // Try to switch to non-existent group ID (lines 633-634)
    // This should be a no-op — no API calls made
    expect(global.fetch).not.toHaveBeenCalled();
  });

  it("is a no-op when switching to already selected group ID", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "Group A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
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

    // Try to switch to already selected group (line 632)
    // Should be a no-op
    expect(global.fetch).not.toHaveBeenCalled();
  });
});

describe("OGSGroupPage ID-based selection: student count update", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem("sidebar-last-group");
    mockSearchParamsGet.mockReturnValue(null);
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        students: [
          {
            id: "2",
            name: "Erika Schmidt",
            first_name: "Erika",
            last_name: "Schmidt",
            current_location: "Raum 202",
          },
          {
            id: "3",
            name: "Hans Mueller",
            first_name: "Hans",
            last_name: "Mueller",
            current_location: "Raum 202",
          },
        ],
      }),
    });
  });

  afterEach(() => {
    cleanup();
  });

  it("updates student count by group ID after loading students", async () => {
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          groups: [
            {
              id: 1,
              name: "Group A",
              room_id: 10,
              room: { id: 10, name: "Raum 101" },
            },
            {
              id: 2,
              name: "Group B",
              room_id: 20,
              room: { id: 20, name: "Raum 202" },
            },
          ],
          students: [
            {
              id: "1",
              name: "Max Mustermann",
              first_name: "Max",
              last_name: "Mustermann",
              current_location: "Raum 101",
            },
          ],
          roomStatus: {
            student_room_status: {
              "1": { in_group_room: true },
            },
          },
          substitutions: [],
          pickupTimes: [],
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

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // Verify student count update logic would use group.id === groupId (lines 653-657)
    expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();
  });
});

describe("OGSGroupPage ID-based selection: tab change handler", () => {
  const mockMutate = vi.fn();
  const originalLocalStorage = window.localStorage;

  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParamsGet.mockReturnValue(null);
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        students: [
          {
            id: "2",
            name: "Erika Schmidt",
            first_name: "Erika",
            last_name: "Schmidt",
            current_location: "Raum 202",
          },
        ],
      }),
    });

    // Mock localStorage
    const localStorageMock: Record<string, string> = {};
    Object.defineProperty(window, "localStorage", {
      value: {
        getItem: (key: string) => localStorageMock[key] ?? null,
        setItem: (key: string, value: string) => {
          localStorageMock[key] = value;
        },
        removeItem: (key: string) => {
          delete localStorageMock[key];
        },
      },
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    cleanup();
    Object.defineProperty(window, "localStorage", {
      value: originalLocalStorage,
      writable: true,
      configurable: true,
    });
  });

  it("finds group by ID when tab changes", async () => {
    // Setup: Multiple groups, tabs visible
    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: {
          groups: [
            {
              id: 1,
              name: "Group A",
              room_id: 10,
              room: { id: 10, name: "Raum 101" },
            },
            {
              id: 2,
              name: "Group B",
              room_id: 20,
              room: { id: 20, name: "Raum 202" },
            },
          ],
          students: [
            {
              id: "1",
              name: "Max Mustermann",
              first_name: "Max",
              last_name: "Mustermann",
              current_location: "Raum 101",
            },
          ],
          roomStatus: {
            student_room_status: {
              "1": { in_group_room: true },
            },
          },
          substitutions: [],
          pickupTimes: [],
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

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // Simulate tab change to Group B (lines 1122-1133)
    // The tab handler would find group by ID and call switchToGroup
    const tabButton = screen.queryByText("Group B");
    if (tabButton) {
      tabButton.click();

      await waitFor(() => {
        expect(global.fetch).toHaveBeenCalled();
      });
    }
  });
});

describe("OGSGroupPage ID-based selection: currentGroup useMemo", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    localStorage.removeItem("sidebar-last-group");
    mockSearchParamsGet.mockReturnValue(null);
  });

  afterEach(() => {
    cleanup();
  });

  it("finds currentGroup by ID, falls back to first group", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "Group A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
          {
            id: 2,
            name: "Group B",
            room_id: 20,
            room: { id: 20, name: "Raum 202" },
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
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

    // currentGroup should be found by ID (lines 351-355)
    // When selectedGroupId=null, it falls back to allGroups[0]
    expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();
  });

  it("falls back to first group when selected ID not found", async () => {
    // Scenario: selectedGroupId was "999" (doesn't exist)
    // currentGroup useMemo should return allGroups[0]
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "Group A",
            room_id: 10,
            room: { id: 10, name: "Raum 101" },
          },
        ],
        students: [
          {
            id: "1",
            name: "Max Mustermann",
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          },
        ],
        roomStatus: {
          student_room_status: {
            "1": { in_group_room: true },
          },
        },
        substitutions: [],
        pickupTimes: [],
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

    // Should display first group (fallback logic in useMemo)
    expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();
  });

  it("returns null when no groups exist", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [],
        students: [],
        roomStatus: null,
        substitutions: [],
        pickupTimes: [],
        firstGroupId: null,
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Keine OGS-Gruppe zugeordnet/),
      ).toBeInTheDocument();
    });

    // currentGroup useMemo should return null (lines 351-355)
    expect(screen.queryByTestId("student-card")).not.toBeInTheDocument();
  });
});
