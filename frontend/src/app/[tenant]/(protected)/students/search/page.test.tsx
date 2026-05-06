import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { TrackingIndicatorsResponse } from "~/lib/active-helpers";
import StudentSearchPage from "./page";

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { token: "test-token" } },
    status: "authenticated",
  })),
}));

// Mock next/navigation
const mockSearchParams = new URLSearchParams();
vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({
    push: vi.fn(),
    replace: vi.fn(),
  })),
  useSearchParams: vi.fn(() => mockSearchParams),
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
  Loading: ({ fullPage }: { fullPage?: boolean }) => (
    <div data-testid={fullPage ? "loading-full" : "loading"}>Loading...</div>
  ),
}));

// Mock Alert component
vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message, type }: { message: string; type: string }) => (
    <div data-testid={`alert-${type}`}>{message}</div>
  ),
}));

// Mock PageHeaderWithSearch
vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({
    filters,
    activeFilters,
    onClearAllFilters,
    search,
  }: {
    filters: Array<{
      id: string;
      value: string;
      onChange: (v: string) => void;
      options?: Array<{ value: string; label: string }>;
    }>;
    activeFilters: Array<{ id: string; label: string }>;
    onClearAllFilters: () => void;
    search: { value: string; onChange: (v: string) => void };
  }) => (
    <div data-testid="page-header">
      <input
        data-testid="search-input"
        value={search.value}
        onChange={(e) => search.onChange(e.target.value)}
      />
      {filters.map((f) => (
        <select
          key={f.id}
          data-testid={`filter-${f.id}`}
          value={f.value}
          onChange={(e) => f.onChange(e.target.value)}
        >
          {f.options ? (
            f.options.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))
          ) : (
            <>
              <option value="all">All</option>
              <option value="anwesend">Anwesend</option>
              <option value="abwesend">Abwesend</option>
              <option value="unterwegs">Unterwegs</option>
              <option value="schulhof">Schulhof</option>
            </>
          )}
        </select>
      ))}
      <div data-testid="active-filters">
        {activeFilters.map(
          (f: { id: string; label: string; onRemove?: () => void }) => (
            <button
              key={f.id}
              data-testid={`active-filter-${f.id}`}
              onClick={f.onRemove}
            >
              {f.label}
            </button>
          ),
        )}
      </div>
      <button data-testid="clear-filters" onClick={onClearAllFilters}>
        Clear
      </button>
    </div>
  ),
}));

// Mock StudentPresenceBadge — wrapper the page now renders instead of
// the bare LocationBadge, so binary-mode tenants can hide detailed labels.
vi.mock("@/components/ui/student-presence-badge", () => ({
  StudentPresenceBadge: ({
    student,
  }: {
    student: { current_location: string };
  }) => <span data-testid="location-badge">{student.current_location}</span>,
}));

// Mock LocationBadge (still referenced transitively by any non-mocked paths)
vi.mock("@/components/ui/location-badge", () => ({
  LocationBadge: ({ student }: { student: { current_location: string } }) => (
    <span data-testid="location-badge">{student.current_location}</span>
  ),
}));

// Mock location helpers — LOCATION_COLORS is consumed by student-card.tsx
// (check-in mode tint), so the mock must expose the brand palette even if
// individual tests don't assert on colors.
vi.mock("~/lib/location-helper", () => ({
  isHomeLocation: (loc: string) => loc === "Zuhause" || loc === "",
  isPresentLocation: (loc: string) =>
    loc !== "Zuhause" &&
    loc !== "" &&
    loc !== "Unterwegs" &&
    loc !== "Schulhof",
  isTransitLocation: (loc: string) => loc === "Unterwegs",
  isSchoolyardLocation: (loc: string) => loc === "Schulhof",
  LOCATION_COLORS: {
    GROUP_ROOM: "#83CD2D",
    OTHER_ROOM: "#5080D8",
    HOME: "#FF3130",
    SCHOOLYARD: "#F78C10",
    TRANSIT: "#D946EF",
    UNKNOWN: "#6B7280",
    SICK: "#EAB308",
    EXCUSED: "#7C3AED",
  },
  LOCATION_STATUSES: {
    PRESENT: "Anwesend",
    HOME: "Zuhause",
    SCHOOLYARD: "Schulhof",
    TRANSIT: "Unterwegs",
    UNKNOWN: "Unbekannt",
    SICK: "Krank",
    EXCUSED: "Entschuldigt",
  },
}));

// Mock school-checkin FAB + hook so existing search tests aren't
// responsible for the new floating mode trigger —
// page.school-checkin.test.tsx covers it dedicatedly.
vi.mock("~/components/students/school-checkin-fab", () => ({
  SchoolCheckinFab: () => <div data-testid="school-checkin-fab" />,
}));

vi.mock("~/lib/hooks/use-school-checkin-mode", () => ({
  useSchoolCheckinMode: () => ({
    isActive: false,
    toggleActive: vi.fn(),
    deactivate: vi.fn(),
    pendingIds: new Set<string>(),
    successCount: 0,
    toggle: vi.fn(),
  }),
  deriveCheckinState: () => "unknown",
}));

// Mock student-helpers
vi.mock("~/lib/student-helpers", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/student-helpers")>();
  return {
    ...actual,
    SCHOOL_YEAR_FILTER_OPTIONS: [
      { value: "all", label: "Alle" },
      { value: "1", label: "1" },
      { value: "2", label: "2" },
      { value: "3", label: "3" },
      { value: "4", label: "4" },
    ],
  };
});

// Mock SSE hook
vi.mock("~/lib/hooks/use-sse", () => ({
  useSSE: vi.fn(() => ({
    status: "connected",
    isConnected: true,
    error: null,
  })),
}));

// Mock SWR hooks - configured per test in beforeEach
vi.mock("~/lib/swr", () => ({
  useImmutableSWR: vi.fn(),
  useSWRAuth: vi.fn(),
  mutate: vi.fn(),
  useTenantMutate: vi.fn(() => vi.fn()),
}));

// Mock API services
const mockStudents = [
  {
    id: "1",
    first_name: "Max",
    second_name: "Mustermann",
    school_class: "1a",
    group_name: "Gruppe A",
    current_location: "Raum 101",
    arrival_time: "08:00",
    pickup_time: "15:30",
    has_full_access: true,
  },
  {
    id: "2",
    first_name: "Anna",
    second_name: "Schmidt",
    school_class: "2b",
    group_name: "Gruppe B",
    current_location: "Zuhause",
    arrival_time: "08:30",
    has_full_access: true,
  },
  {
    id: "3",
    first_name: "Tom",
    second_name: "Weber",
    school_class: "1a",
    group_name: "Gruppe A",
    current_location: "Unterwegs",
    arrival_time: "08:15",
    pickup_time: "16:00",
    has_full_access: true,
  },
  {
    id: "4",
    first_name: "Lisa",
    second_name: "Müller",
    school_class: "3c",
    group_name: "Gruppe C",
    current_location: "Schulhof",
    arrival_time: "09:00",
    has_full_access: true,
  },
];

vi.mock("~/lib/api", () => ({
  studentService: {
    getStudents: vi.fn(() =>
      Promise.resolve({
        students: mockStudents,
      }),
    ),
  },
  groupService: {
    getGroups: vi.fn(() =>
      Promise.resolve([
        { id: "1", name: "Gruppe A" },
        { id: "2", name: "Gruppe B" },
        { id: "3", name: "Gruppe C" },
      ]),
    ),
  },
  roomService: {
    getRooms: vi.fn(() =>
      Promise.resolve([
        { id: "101", name: "Raum 101", isOccupied: true },
        { id: "102", name: "Raum 102", isOccupied: false },
      ]),
    ),
  },
}));

vi.mock("~/lib/usercontext-api", () => ({
  userContextService: {
    getMyEducationalGroups: vi.fn(() => Promise.resolve([])),
    getMySupervisedGroups: vi.fn(() => Promise.resolve([])),
  },
}));

describe("StudentSearchPage", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    mockSearchParams.delete("status");

    // Reset useSession mock to authenticated state
    const sessionModule = await import("next-auth/react");
    vi.mocked(sessionModule.useSession).mockReturnValue({
      data: { user: { token: "test-token" }, expires: "2099-01-01" },
      status: "authenticated",
      update: vi.fn(),
    } as unknown as ReturnType<typeof sessionModule.useSession>);

    // Reset SWR mock data for each test
    const swrModule = await import("~/lib/swr");
    vi.mocked(swrModule.useImmutableSWR).mockImplementation((key) => {
      if (key === "search-rooms-list") {
        return {
          data: [
            { id: "101", name: "Raum 101", isOccupied: true },
            { id: "102", name: "Raum 102", isOccupied: false },
          ],
          isLoading: false,
          error: null,
        } as ReturnType<typeof swrModule.useImmutableSWR>;
      }

      return {
        data: [
          { id: "1", name: "Gruppe A" },
          { id: "2", name: "Gruppe B" },
          { id: "3", name: "Gruppe C" },
        ],
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useImmutableSWR>;
    });
    vi.mocked(swrModule.useSWRAuth).mockReturnValue({
      data: { students: mockStudents },
      isLoading: false,
      error: null,
    } as ReturnType<typeof swrModule.useSWRAuth>);
  });

  afterEach(() => {
    cleanup();
  });

  describe("URL Parameter Handling", () => {
    it("defaults to 'all' when no status param is present", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        const attendanceFilter = screen.getByTestId("filter-attendance");
        expect(attendanceFilter).toHaveValue("all");
      });
    });

    it("reads 'anwesend' status from URL params", async () => {
      mockSearchParams.set("status", "anwesend");

      render(<StudentSearchPage />);

      await waitFor(() => {
        const attendanceFilter = screen.getByTestId("filter-attendance");
        expect(attendanceFilter).toHaveValue("anwesend");
      });
    });

    it("reads 'unterwegs' status from URL params", async () => {
      mockSearchParams.set("status", "unterwegs");

      render(<StudentSearchPage />);

      await waitFor(() => {
        const attendanceFilter = screen.getByTestId("filter-attendance");
        expect(attendanceFilter).toHaveValue("unterwegs");
      });
    });

    it("reads 'schulhof' status from URL params", async () => {
      mockSearchParams.set("status", "schulhof");

      render(<StudentSearchPage />);

      await waitFor(() => {
        const attendanceFilter = screen.getByTestId("filter-attendance");
        expect(attendanceFilter).toHaveValue("schulhof");
      });
    });

    it("reads 'abwesend' status from URL params", async () => {
      mockSearchParams.set("status", "abwesend");

      render(<StudentSearchPage />);

      await waitFor(() => {
        const attendanceFilter = screen.getByTestId("filter-attendance");
        expect(attendanceFilter).toHaveValue("abwesend");
      });
    });

    it("falls back to 'all' for invalid status param", async () => {
      mockSearchParams.set("status", "invalid_status");

      render(<StudentSearchPage />);

      await waitFor(() => {
        const attendanceFilter = screen.getByTestId("filter-attendance");
        expect(attendanceFilter).toHaveValue("all");
      });
    });
  });

  describe("Client-Side Filtering", () => {
    it("shows all students when filter is 'all'", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        // All 4 students should be visible (check by first names)
        expect(screen.getByText("Max")).toBeInTheDocument();
        expect(screen.getByText("Anna")).toBeInTheDocument();
        expect(screen.getByText("Tom")).toBeInTheDocument();
        expect(screen.getByText("Lisa")).toBeInTheDocument();
      });
    });

    it("filters to show only present students when 'anwesend' is selected", async () => {
      mockSearchParams.set("status", "anwesend");

      render(<StudentSearchPage />);

      await waitFor(() => {
        // Max (Raum 101), Tom (Unterwegs), Lisa (Schulhof) are on-site
        expect(screen.getByText("Max")).toBeInTheDocument();
        expect(screen.getByText("Tom")).toBeInTheDocument();
        expect(screen.getByText("Lisa")).toBeInTheDocument();
        // Anna (Zuhause) should be filtered out
        expect(screen.queryByText("Anna")).not.toBeInTheDocument();
      });
    });

    it("filters to show only home students when 'abwesend' is selected", async () => {
      mockSearchParams.set("status", "abwesend");

      render(<StudentSearchPage />);

      await waitFor(() => {
        // Only Anna (Zuhause) should be visible
        expect(screen.getByText("Anna")).toBeInTheDocument();
        expect(screen.queryByText("Max")).not.toBeInTheDocument();
        expect(screen.queryByText("Tom")).not.toBeInTheDocument();
        expect(screen.queryByText("Lisa")).not.toBeInTheDocument();
      });
    });

    it("filters to show only transit students when 'unterwegs' is selected", async () => {
      mockSearchParams.set("status", "unterwegs");

      render(<StudentSearchPage />);

      await waitFor(() => {
        // Only Tom (Unterwegs) should be visible
        expect(screen.getByText("Tom")).toBeInTheDocument();
        expect(screen.queryByText("Max")).not.toBeInTheDocument();
        expect(screen.queryByText("Anna")).not.toBeInTheDocument();
        expect(screen.queryByText("Lisa")).not.toBeInTheDocument();
      });
    });

    it("filters to show only schoolyard students when 'schulhof' is selected", async () => {
      mockSearchParams.set("status", "schulhof");

      render(<StudentSearchPage />);

      await waitFor(() => {
        // Only Lisa (Schulhof) should be visible
        expect(screen.getByText("Lisa")).toBeInTheDocument();
        expect(screen.queryByText("Max")).not.toBeInTheDocument();
        expect(screen.queryByText("Anna")).not.toBeInTheDocument();
        expect(screen.queryByText("Tom")).not.toBeInTheDocument();
      });
    });
  });

  describe("Year Filtering", () => {
    it("renders the school year filter as a stage dropdown", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-year")).toBeInTheDocument();
      });

      expect(
        screen.getByRole("option", { name: "Alle Stufen" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("option", { name: "Stufe 1" }),
      ).toBeInTheDocument();
    });

    it("filters students by school year when year filter changes", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        // All 4 students should be visible initially
        expect(screen.getByText("Max")).toBeInTheDocument();
        expect(screen.getByText("Anna")).toBeInTheDocument();
        expect(screen.getByText("Tom")).toBeInTheDocument();
        expect(screen.getByText("Lisa")).toBeInTheDocument();
      });

      // Change year filter to "1" (should show only Max and Tom with class 1a)
      const yearFilter = screen.getByTestId("filter-year");
      fireEvent.change(yearFilter, { target: { value: "1" } });

      await waitFor(() => {
        // Max (1a) and Tom (1a) should be visible
        expect(screen.getByText("Max")).toBeInTheDocument();
        expect(screen.getByText("Tom")).toBeInTheDocument();
        // Anna (2b) and Lisa (3c) should be filtered out
        expect(screen.queryByText("Anna")).not.toBeInTheDocument();
        expect(screen.queryByText("Lisa")).not.toBeInTheDocument();
      });
    });
  });

  describe("Loading States", () => {
    it("shows loading state when session is loading", async () => {
      const useSession = await import("next-auth/react");
      vi.mocked(useSession.useSession).mockReturnValue({
        data: null,
        status: "loading",
        update: vi.fn(),
      });

      render(<StudentSearchPage />);

      expect(screen.getByTestId("loading")).toBeInTheDocument();
    });

    it("shows loading state while fetching students", async () => {
      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useSWRAuth).mockReturnValue({
        data: undefined,
        isLoading: true,
        // IMPORTANT: Use undefined, not null. The page checks `error !== undefined`
        // to determine if a fetch has completed. null !== undefined is true,
        // which would incorrectly mark the fetch as complete.
        error: undefined,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("loading")).toBeInTheDocument();
      });
    });
  });

  describe("Error Handling", () => {
    it("renders 403 permission denied error message", async () => {
      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useSWRAuth).mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error("403 Forbidden"),
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        // Check that at least one element contains the error message
        const errorElements = screen.getAllByText(/keine Berechtigung/i);
        expect(errorElements.length).toBeGreaterThan(0);
      });
    });

    it("renders 401 session expired error message", async () => {
      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useSWRAuth).mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error("401 Unauthorized"),
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        const errorElements = screen.getAllByText(/Sitzung ist abgelaufen/i);
        expect(errorElements.length).toBeGreaterThan(0);
      });
    });

    it("renders generic error for other API errors", async () => {
      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useSWRAuth).mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error("Network Error"),
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        const errorElements = screen.getAllByText(
          /Fehler beim Laden der Schülerdaten/i,
        );
        expect(errorElements.length).toBeGreaterThan(0);
      });
    });
  });

  describe("Empty State", () => {
    it("shows empty state when no students match filters", async () => {
      // Mock SWR to return empty students
      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useSWRAuth).mockReturnValue({
        data: { students: [] },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText("Keine Schüler gefunden")).toBeInTheDocument();
      });
    });
  });

  describe("Clear Filters", () => {
    it("clears all filters when clear button is clicked", async () => {
      mockSearchParams.set("status", "unterwegs");

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-attendance")).toHaveValue(
          "unterwegs",
        );
      });

      const clearButton = screen.getByTestId("clear-filters");
      fireEvent.click(clearButton);

      await waitFor(() => {
        expect(screen.getByTestId("filter-attendance")).toHaveValue("all");
      });
    });
  });

  describe("Student Card Navigation", () => {
    it("navigates to student detail when card is clicked", async () => {
      const mockPush = vi.fn();
      const useRouter = await import("next/navigation");
      vi.mocked(useRouter.useRouter).mockReturnValue({
        push: mockPush,
        replace: vi.fn(),
        back: vi.fn(),
        forward: vi.fn(),
        refresh: vi.fn(),
        prefetch: vi.fn(),
      });

      render(<StudentSearchPage />);

      // Wait for students to load - StudentCard displays first_name in h3
      await waitFor(() => {
        expect(screen.getByText("Max")).toBeInTheDocument();
      });

      // Find the student card (button with role) for "Max"
      const studentCard = screen.getByText("Max").closest("button");
      if (studentCard) {
        fireEvent.click(studentCard);
      }

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith(
          "/test-tenant/students/1?from=/students/search",
        );
      });
    });
  });

  describe("SWR Fetcher Execution", () => {
    it("executes the groups SWR fetcher successfully", async () => {
      const groupService = await import("~/lib/api");
      const mockGetGroups = vi.fn().mockResolvedValue([
        { id: "1", name: "Test Group A" },
        { id: "2", name: "Test Group B" },
      ]);
      vi.mocked(groupService.groupService.getGroups).mockImplementation(
        mockGetGroups,
      );

      let capturedGroupsFetcher: (() => Promise<unknown>) | null = null;

      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useImmutableSWR).mockImplementation(
        (key, fetcher) => {
          if (key === "search-groups-list" && fetcher) {
            capturedGroupsFetcher = fetcher as () => Promise<unknown>;
          }
          return {
            data: [
              { id: "1", name: "Test Group A" },
              { id: "2", name: "Test Group B" },
            ],
            isLoading: false,
            error: null,
          } as ReturnType<typeof swrModule.useImmutableSWR>;
        },
      );

      vi.mocked(swrModule.useSWRAuth).mockReturnValue({
        data: { students: mockStudents },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      await act(async () => {
        render(<StudentSearchPage />);
      });

      // Execute the captured fetcher to cover lines 93-103
      expect(capturedGroupsFetcher).not.toBeNull();
      const result: unknown = await act(async () => {
        return await (
          capturedGroupsFetcher as unknown as () => Promise<unknown>
        )();
      });
      expect(result).toEqual([
        { id: "1", name: "Test Group A" },
        { id: "2", name: "Test Group B" },
      ]);
      expect(mockGetGroups).toHaveBeenCalled();
    });

    it("handles groups fetcher error gracefully", async () => {
      const groupService = await import("~/lib/api");
      const mockGetGroups = vi
        .fn()
        .mockRejectedValue(new Error("Permission denied"));
      vi.mocked(groupService.groupService.getGroups).mockImplementation(
        mockGetGroups,
      );

      let capturedGroupsFetcher: (() => Promise<unknown>) | null = null;

      const consoleWarnSpy = vi
        .spyOn(console, "warn")
        .mockImplementation(() => undefined);

      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useImmutableSWR).mockImplementation(
        (key, fetcher) => {
          if (key === "search-groups-list" && fetcher) {
            capturedGroupsFetcher = fetcher as () => Promise<unknown>;
          }
          return {
            data: [],
            isLoading: false,
            error: null,
          } as ReturnType<typeof swrModule.useImmutableSWR>;
        },
      );

      vi.mocked(swrModule.useSWRAuth).mockReturnValue({
        data: { students: mockStudents },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      await act(async () => {
        render(<StudentSearchPage />);
      });

      // Execute the captured fetcher to cover catch block (lines 98-101)
      expect(capturedGroupsFetcher).not.toBeNull();
      const result: unknown = await act(async () => {
        return await (
          capturedGroupsFetcher as unknown as () => Promise<unknown>
        )();
      });
      // Should return empty array on error
      expect(result).toEqual([]);
      expect(consoleWarnSpy).toHaveBeenCalledWith(
        "could not load groups for filter",
        undefined,
      );

      consoleWarnSpy.mockRestore();
    });

    it("executes the students SWR fetcher", async () => {
      const studentService = await import("~/lib/api");
      const mockGetStudents = vi.fn().mockResolvedValue({
        students: [{ id: "1", first_name: "Test", second_name: "Student" }],
      });
      vi.mocked(studentService.studentService.getStudents).mockImplementation(
        mockGetStudents,
      );

      let capturedStudentsFetcher: (() => Promise<unknown>) | null = null;

      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useImmutableSWR).mockReturnValue({
        data: [],
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useImmutableSWR>);

      vi.mocked(swrModule.useSWRAuth).mockImplementation((key, fetcher) => {
        // Capture the students fetcher when the key contains "search-students"
        // Note: key is null until groupsLoaded state becomes true after useEffect runs
        if (
          typeof key === "string" &&
          key.includes("search-students") &&
          fetcher
        ) {
          capturedStudentsFetcher = fetcher as () => Promise<unknown>;
        }
        return {
          data: { students: mockStudents },
          isLoading: false,
          error: null,
        } as ReturnType<typeof swrModule.useSWRAuth>;
      });

      render(<StudentSearchPage />);

      // Wait for the component to re-render after groupsLoaded becomes true
      // This happens in a useEffect, so we need to wait for it
      await waitFor(() => {
        expect(capturedStudentsFetcher).not.toBeNull();
      });

      // Execute the captured fetcher to cover lines 117-128
      const result: unknown = await (
        capturedStudentsFetcher as unknown as () => Promise<unknown>
      )();
      expect(result).toEqual({
        students: [{ id: "1", first_name: "Test", second_name: "Student" }],
      });
      expect(mockGetStudents).toHaveBeenCalled();
    });
  });

  describe("Error Display Rendering", () => {
    // Fix P3 regression test: Error heading now uses errorType instead of substring matching
    it("renders 'Keine Berechtigung' heading for 403 errors (P3 fix)", async () => {
      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useSWRAuth).mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error("403 Forbidden"),
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        // The transformed error message for 403
        expect(
          screen.getAllByText(
            /Du hast keine Berechtigung, Schülerdaten anzuzeigen/,
          ).length,
        ).toBeGreaterThan(0);
        // P3 FIX: The error heading should now be "Keine Berechtigung" (not "Fehler")
        // because we use errorType === "permission" instead of error.includes("403")
        expect(screen.getByText("Keine Berechtigung")).toBeInTheDocument();
      });
    });

    it("renders 'Fehler' heading for 401 session errors", async () => {
      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useSWRAuth).mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error("401 Unauthorized"),
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        // The transformed error message for 401
        expect(
          screen.getAllByText(/Sitzung ist abgelaufen/).length,
        ).toBeGreaterThan(0);
        // Session errors still show generic "Fehler" heading
        expect(screen.getByText("Fehler")).toBeInTheDocument();
      });
    });

    it("renders generic error heading for non-403/401 errors", async () => {
      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useSWRAuth).mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error("500 Internal Server Error"),
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        // The generic error message
        expect(
          screen.getAllByText(/Fehler beim Laden der Schülerdaten/).length,
        ).toBeGreaterThan(0);
        // The error heading
        expect(screen.getByText("Fehler")).toBeInTheDocument();
      });
    });
  });

  // P1 Regression Tests: Unauthenticated users should be redirected (not see empty state)
  describe("Authentication Redirect Handling (P1 fix)", () => {
    it("redirects to home when user is unauthenticated", async () => {
      const mockPush = vi.fn();
      const useRouter = await import("next/navigation");
      vi.mocked(useRouter.useRouter).mockReturnValue({
        push: mockPush,
        replace: vi.fn(),
        back: vi.fn(),
        forward: vi.fn(),
        refresh: vi.fn(),
        prefetch: vi.fn(),
      });

      const useSession = await import("next-auth/react");
      // Simulate NextAuth's useSession with required: true - it calls onUnauthenticated callback
      vi.mocked(useSession.useSession).mockImplementation((options) => {
        // When required: true and user is unauthenticated, NextAuth calls the callback
        if (
          options &&
          typeof options === "object" &&
          "required" in options &&
          options.required
        ) {
          const opts = options as { onUnauthenticated?: () => void };
          if (opts.onUnauthenticated) {
            opts.onUnauthenticated();
          }
        }
        return {
          data: null,
          status: "unauthenticated",
          update: vi.fn(),
        };
      });

      // SWR won't fetch when unauthenticated
      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useSWRAuth).mockReturnValue({
        data: undefined,
        isLoading: false,
        error: undefined,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      // P1 FIX: Should redirect to home page, NOT show empty state or error message
      expect(mockPush).toHaveBeenCalledWith("/test-tenant/");
    });

    it("shows loading state during auth check (no empty state flash)", async () => {
      const useSession = await import("next-auth/react");
      vi.mocked(useSession.useSession).mockReturnValue({
        data: null,
        status: "loading", // Session check in progress
        update: vi.fn(),
      });

      // SWR won't fetch during auth loading
      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useSWRAuth).mockReturnValue({
        data: undefined,
        isLoading: false,
        error: undefined,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      // Should show loading, NOT empty state
      expect(screen.getByTestId("loading")).toBeInTheDocument();
      expect(
        screen.queryByText("Keine Schüler gefunden"),
      ).not.toBeInTheDocument();
    });

    it("does NOT redirect when user is authenticated with token", async () => {
      const mockPush = vi.fn();
      const useRouter = await import("next/navigation");
      vi.mocked(useRouter.useRouter).mockReturnValue({
        push: mockPush,
        replace: vi.fn(),
        back: vi.fn(),
        forward: vi.fn(),
        refresh: vi.fn(),
        prefetch: vi.fn(),
      });

      // Default authenticated state with token
      const useSession = await import("next-auth/react");
      vi.mocked(useSession.useSession).mockReturnValue({
        data: { user: { token: "valid-token" }, expires: "2099-01-01" },
        status: "authenticated",
        update: vi.fn(),
      } as unknown as ReturnType<typeof useSession.useSession>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        // Should render page header (meaning component rendered normally)
        expect(screen.getByTestId("page-header")).toBeInTheDocument();
      });

      // Should NOT have redirected to home
      expect(mockPush).not.toHaveBeenCalledWith("/test-tenant/");
    });
  });

  // P2 Regression Tests: Empty state should not flash before first fetch
  describe("Empty State Flash Prevention (P2 fix)", () => {
    it("shows loading state before first fetch completes (not empty state)", async () => {
      // Simulate the initial state before groupsLoaded becomes true
      const swrModule = await import("~/lib/swr");

      // Groups haven't loaded yet, so studentsCacheKey is null
      // SWR returns undefined data (not yet fetched)
      vi.mocked(swrModule.useSWRAuth).mockReturnValue({
        data: undefined, // No data yet - first fetch hasn't completed
        isLoading: true, // SWR is loading students
        error: undefined,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      vi.mocked(swrModule.useImmutableSWR).mockReturnValue({
        data: undefined, // Groups also loading
        isLoading: true,
        error: null,
      } as ReturnType<typeof swrModule.useImmutableSWR>);

      await act(async () => {
        render(<StudentSearchPage />);
      });

      // P2 FIX: Should show loading spinner, NOT "Keine Schüler gefunden"
      // because we're in initialization phase (groupsLoaded = false)
      expect(screen.getByTestId("loading")).toBeInTheDocument();
      expect(
        screen.queryByText("Keine Schüler gefunden"),
      ).not.toBeInTheDocument();
    });

    it("shows empty state only AFTER first fetch returns empty results", async () => {
      const swrModule = await import("~/lib/swr");

      // Groups have loaded
      vi.mocked(swrModule.useImmutableSWR).mockReturnValue({
        data: [{ id: "1", name: "Gruppe A" }],
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useImmutableSWR>);

      // Students fetch completed with empty results (hasFetchedOnce = true)
      vi.mocked(swrModule.useSWRAuth).mockReturnValue({
        data: { students: [] }, // Empty results from completed fetch
        isLoading: false,
        error: undefined,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        // NOW it's appropriate to show empty state
        expect(screen.getByText("Keine Schüler gefunden")).toBeInTheDocument();
      });
    });
  });

  describe("Active Filter Removal", () => {
    it("removes attendance filter when active filter chip is clicked", async () => {
      mockSearchParams.set("status", "anwesend");

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(
          screen.getByTestId("active-filter-attendance"),
        ).toBeInTheDocument();
      });

      // Click the active filter to remove it
      const activeFilter = screen.getByTestId("active-filter-attendance");
      fireEvent.click(activeFilter);

      await waitFor(() => {
        expect(screen.getByTestId("filter-attendance")).toHaveValue("all");
        expect(
          screen.queryByTestId("active-filter-attendance"),
        ).not.toBeInTheDocument();
      });
    });

    it("removes group filter when active filter chip is clicked", async () => {
      render(<StudentSearchPage />);

      // First set a group filter
      await waitFor(() => {
        expect(screen.getByTestId("filter-group")).toBeInTheDocument();
      });

      const groupFilter = screen.getByTestId("filter-group");
      fireEvent.change(groupFilter, { target: { value: "1" } });

      await waitFor(() => {
        expect(screen.getByTestId("active-filter-group")).toBeInTheDocument();
      });

      // Click the active filter to remove it
      const activeFilter = screen.getByTestId("active-filter-group");
      fireEvent.click(activeFilter);

      await waitFor(() => {
        expect(screen.getByTestId("filter-group")).toHaveValue("");
        expect(
          screen.queryByTestId("active-filter-group"),
        ).not.toBeInTheDocument();
      });
    });
  });

  describe("Pickup Time Filtering", () => {
    it("filters students by specific pickup time", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText("Max")).toBeInTheDocument();
      });

      // Filter by 15:30 — only Max has this pickup time
      const pickupFilter = screen.getByTestId("filter-pickupTime");
      fireEvent.change(pickupFilter, { target: { value: "15:30" } });

      await waitFor(() => {
        expect(screen.getByText("Max")).toBeInTheDocument();
        expect(screen.queryByText("Anna")).not.toBeInTheDocument();
        expect(screen.queryByText("Tom")).not.toBeInTheDocument();
        expect(screen.queryByText("Lisa")).not.toBeInTheDocument();
      });
    });

    it("filters students with 'none' to show only students without pickup time", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText("Max")).toBeInTheDocument();
      });

      // Filter by "none" — Anna and Lisa have no pickup_time
      const pickupFilter = screen.getByTestId("filter-pickupTime");
      fireEvent.change(pickupFilter, { target: { value: "none" } });

      await waitFor(() => {
        // Anna has no pickup_time and has_full_access=true
        expect(screen.getByText("Anna")).toBeInTheDocument();
        // Lisa has no pickup_time and has_full_access=true
        expect(screen.getByText("Lisa")).toBeInTheDocument();
        // Max and Tom have pickup_time set, so they should be filtered out
        expect(screen.queryByText("Max")).not.toBeInTheDocument();
        expect(screen.queryByText("Tom")).not.toBeInTheDocument();
      });
    });

    it("hides redacted students (has_full_access=false) when pickup time filter is active", async () => {
      // Create mock students where one has has_full_access=false
      const studentsWithRedacted = [
        {
          id: "10",
          first_name: "Visible",
          second_name: "Student",
          school_class: "1a",
          group_name: "Gruppe A",
          current_location: "Raum 101",
          has_full_access: true,
        },
        {
          id: "11",
          first_name: "Redacted",
          second_name: "Student",
          school_class: "1a",
          group_name: "Gruppe A",
          current_location: "Raum 102",
          has_full_access: false,
        },
      ];

      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useSWRAuth).mockReturnValue({
        data: { students: studentsWithRedacted },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText("Visible")).toBeInTheDocument();
        expect(screen.getByText("Redacted")).toBeInTheDocument();
      });

      // Apply "none" pickup time filter — redacted student should be excluded (GDPR)
      const pickupFilter = screen.getByTestId("filter-pickupTime");
      fireEvent.change(pickupFilter, { target: { value: "none" } });

      await waitFor(() => {
        expect(screen.getByText("Visible")).toBeInTheDocument();
        // Redacted student must NOT appear — has_full_access=false means we
        // can't know their pickup status, so they're excluded from filtering
        expect(screen.queryByText("Redacted")).not.toBeInTheDocument();
      });
    });

    it("shows active filter chips with correct labels and clears via chip click or clear-all", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-pickupTime")).toBeInTheDocument();
      });

      const pickupFilter = screen.getByTestId("filter-pickupTime");

      // Verify "Keine Abholzeit" chip label for "none"
      fireEvent.change(pickupFilter, { target: { value: "none" } });
      await waitFor(() => {
        expect(
          screen.getByTestId("active-filter-pickupTime"),
        ).toHaveTextContent("Keine Abholzeit");
      });

      // Switch to specific time — verify chip label and clear-all
      fireEvent.change(pickupFilter, { target: { value: "15:30" } });
      await waitFor(() => {
        expect(
          screen.getByTestId("active-filter-pickupTime"),
        ).toHaveTextContent("Abholzeit 15:30 Uhr");
      });

      // Clear all filters
      fireEvent.click(screen.getByTestId("clear-filters"));
      await waitFor(() => {
        expect(screen.getByTestId("filter-pickupTime")).toHaveValue("all");
        expect(
          screen.queryByTestId("active-filter-pickupTime"),
        ).not.toBeInTheDocument();
      });
    });

    it("displays pickup time on student cards when present", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        // Max has pickup_time="15:30"
        expect(screen.getByText("Max")).toBeInTheDocument();
      });

      // Verify pickup time text appears for students with pickup_time
      await waitFor(() => {
        expect(screen.getByText("Abholzeit: 15:30 Uhr")).toBeInTheDocument();
        expect(screen.getByText("Abholzeit: 16:00 Uhr")).toBeInTheDocument();
      });
    });

    it("displays fallback pickup row when student has no pickup_time but has full access", async () => {
      // Use only one student without pickup_time
      const studentsNoPickup = [
        {
          id: "20",
          first_name: "NoPickup",
          second_name: "Student",
          school_class: "2a",
          group_name: "Gruppe A",
          current_location: "Raum 101",
          has_full_access: true,
        },
      ];

      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useSWRAuth).mockReturnValue({
        data: { students: studentsNoPickup },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText("NoPickup")).toBeInTheDocument();
      });

      // Should show "Abholzeit: —" fallback for students with full access but no pickup time
      expect(screen.getByText("Abholzeit: —")).toBeInTheDocument();
    });
  });

  describe("Tracking Filter (Aktivitäten heute)", () => {
    // Fixture: two students, two configured labels; student "1" completed
    // Hausaufgaben, student "2" has done nothing yet.
    const trackingStudents = [
      {
        id: "1",
        first_name: "Felix",
        second_name: "Schneider",
        school_class: "1a",
        group_name: "Sterne",
        current_location: "Raum 101",
        has_full_access: true,
      },
      {
        id: "2",
        first_name: "Emma",
        second_name: "Meyer",
        school_class: "1a",
        group_name: "Sterne",
        current_location: "Raum 101",
        has_full_access: true,
      },
    ];

    const trackingTwoLabels: TrackingIndicatorsResponse = {
      labels: ["Hausaufgaben", "Mensa"],
      results: {
        "1": [true, false],
        "2": [false, false],
      },
    };

    // Configures useSWRAuth to return students for the students key and
    // the provided tracking response for the tracking-indicators key.
    async function primeTracking(opts: {
      tracking?: TrackingIndicatorsResponse;
      students?: typeof trackingStudents;
    }) {
      const swrModule = await import("~/lib/swr");
      const studentsResult = {
        data: { students: opts.students ?? trackingStudents },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>;
      const trackingResult = {
        data: opts.tracking,
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>;
      vi.mocked(swrModule.useSWRAuth).mockImplementation((key: unknown) => {
        if (typeof key === "string" && key.startsWith("tracking-indicators-")) {
          return trackingResult;
        }
        return studentsResult;
      });
    }

    it("does not render the tracking dropdown when no labels are configured", async () => {
      await primeTracking({ tracking: { labels: [], results: {} } });
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(screen.getByText("Felix")).toBeInTheDocument(),
      );
      expect(screen.queryByTestId("filter-tracking")).not.toBeInTheDocument();
    });

    it("renders the dropdown with Alle / per-label / Noch nichts erledigt options", async () => {
      await primeTracking({ tracking: trackingTwoLabels });
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(screen.getByTestId("filter-tracking")).toBeInTheDocument(),
      );
      const select = screen.getByTestId("filter-tracking");
      expect(select.querySelector('option[value="all"]')).toHaveTextContent(
        "Alle Aktivitäten heute",
      );
      expect(
        select.querySelector('option[value="missing:0"]'),
      ).toHaveTextContent("Noch nicht in Hausaufgaben");
      expect(
        select.querySelector('option[value="missing:1"]'),
      ).toHaveTextContent("Noch nicht in Mensa");
      expect(
        select.querySelector('option[value="none_visited"]'),
      ).toHaveTextContent("Noch nichts erledigt");
    });

    it("selecting 'missing:0' hides students who already completed that label", async () => {
      await primeTracking({ tracking: trackingTwoLabels });
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(screen.getByText("Felix")).toBeInTheDocument(),
      );

      fireEvent.change(screen.getByTestId("filter-tracking"), {
        target: { value: "missing:0" },
      });

      await waitFor(() => {
        // Felix completed Hausaufgaben (results[0] === true) → hidden.
        expect(screen.queryByText("Felix")).not.toBeInTheDocument();
        // Emma hasn't → visible.
        expect(screen.getByText("Emma")).toBeInTheDocument();
      });
    });

    it("'none_visited' keeps only students missing every configured indicator", async () => {
      await primeTracking({ tracking: trackingTwoLabels });
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(screen.getByText("Felix")).toBeInTheDocument(),
      );

      fireEvent.change(screen.getByTestId("filter-tracking"), {
        target: { value: "none_visited" },
      });

      await waitFor(() => {
        expect(screen.queryByText("Felix")).not.toBeInTheDocument();
        expect(screen.getByText("Emma")).toBeInTheDocument();
      });
    });

    it("shows the chip with the correct label and clears it when clicked", async () => {
      await primeTracking({ tracking: trackingTwoLabels });
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(screen.getByTestId("filter-tracking")).toBeInTheDocument(),
      );

      fireEvent.change(screen.getByTestId("filter-tracking"), {
        target: { value: "missing:1" },
      });

      await waitFor(() => {
        expect(screen.getByTestId("active-filter-tracking")).toHaveTextContent(
          "Noch nicht in Mensa",
        );
      });

      fireEvent.click(screen.getByTestId("active-filter-tracking"));

      await waitFor(() => {
        expect(
          screen.queryByTestId("active-filter-tracking"),
        ).not.toBeInTheDocument();
        expect(screen.getByTestId("filter-tracking")).toHaveValue("all");
      });
    });

    it("'Alle zurücksetzen' clears the tracking filter", async () => {
      await primeTracking({ tracking: trackingTwoLabels });
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(screen.getByTestId("filter-tracking")).toBeInTheDocument(),
      );

      fireEvent.change(screen.getByTestId("filter-tracking"), {
        target: { value: "none_visited" },
      });
      await waitFor(() =>
        expect(
          screen.getByTestId("active-filter-tracking"),
        ).toBeInTheDocument(),
      );

      fireEvent.click(screen.getByTestId("clear-filters"));

      await waitFor(() => {
        expect(
          screen.queryByTestId("active-filter-tracking"),
        ).not.toBeInTheDocument();
        expect(screen.getByTestId("filter-tracking")).toHaveValue("all");
      });
    });

    it("hides redacted students (has_full_access === false) while a tracking filter is active", async () => {
      const mixed = [
        trackingStudents[0],
        {
          id: "9",
          first_name: "Redacted",
          second_name: "Kid",
          school_class: "1a",
          group_name: "Sterne",
          current_location: "Raum 101",
          has_full_access: false,
        },
      ];
      // Only include tracking data for id "1"; redacted student has no row.
      await primeTracking({
        tracking: {
          labels: ["Hausaufgaben"],
          results: { "1": [false] },
        },
        students: mixed as typeof trackingStudents,
      });
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(screen.getByText("Redacted")).toBeInTheDocument(),
      );

      // Initially both visible.
      expect(screen.getByText("Felix")).toBeInTheDocument();
      expect(screen.getByText("Redacted")).toBeInTheDocument();

      fireEvent.change(screen.getByTestId("filter-tracking"), {
        target: { value: "missing:0" },
      });

      await waitFor(() => {
        // Felix missing HA → shown.
        expect(screen.getByText("Felix")).toBeInTheDocument();
        // Redacted kid hidden — no tracking data AND has_full_access false.
        expect(screen.queryByText("Redacted")).not.toBeInTheDocument();
      });
    });
  });

  describe("Arrival sort mode", () => {
    it("re-sorts the list when switching sort=arrival", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => expect(screen.getByText("Max")).toBeInTheDocument());

      // Flip to arrival sort mode — component should re-render without error.
      fireEvent.change(screen.getByTestId("filter-sort"), {
        target: { value: "arrival" },
      });

      await waitFor(() =>
        expect(screen.getByTestId("filter-sort")).toHaveValue("arrival"),
      );

      // All four default students still present post-sort.
      expect(screen.getByText("Max")).toBeInTheDocument();
      expect(screen.getByText("Anna")).toBeInTheDocument();
      expect(screen.getByText("Tom")).toBeInTheDocument();
      expect(screen.getByText("Lisa")).toBeInTheDocument();
    });

    it("offers pickup sorting as a dedicated sort mode", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => expect(screen.getByText("Max")).toBeInTheDocument());

      fireEvent.change(screen.getByTestId("filter-sort"), {
        target: { value: "pickup" },
      });

      await waitFor(() =>
        expect(screen.getByTestId("filter-sort")).toHaveValue("pickup"),
      );
      expect(screen.getByText("Max")).toBeInTheDocument();
      expect(screen.getByText("Tom")).toBeInTheDocument();
    });

    it("filters by a specific arrival time", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => expect(screen.getByText("Max")).toBeInTheDocument());

      fireEvent.change(screen.getByTestId("filter-arrivalTime"), {
        target: { value: "08:15" },
      });

      await waitFor(() => {
        expect(screen.getByText("Tom")).toBeInTheDocument();
      });
      expect(screen.queryByText("Max")).not.toBeInTheDocument();
      expect(screen.queryByText("Anna")).not.toBeInTheDocument();
      expect(screen.queryByText("Lisa")).not.toBeInTheDocument();
    });

    it("groups the result list when a grouping mode is selected", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => expect(screen.getByText("Max")).toBeInTheDocument());

      fireEvent.change(screen.getByTestId("filter-groupMode"), {
        target: { value: "status" },
      });

      await waitFor(() => {
        expect(screen.getAllByTestId("student-group")).toHaveLength(4);
      });
      expect(screen.getAllByText("Anwesend").length).toBeGreaterThan(0);
      expect(screen.getAllByText("Abwesend").length).toBeGreaterThan(0);
    });

    it("shows the room filter even without a room deep-link", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-room")).toBeInTheDocument();
      });
      expect(
        screen.getByRole("option", { name: "Raum 101" }),
      ).toBeInTheDocument();
    });
  });
});
