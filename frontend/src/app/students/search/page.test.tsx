import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
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

// Mock ResponsiveLayout
vi.mock("~/components/dashboard", () => ({
  ResponsiveLayout: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-layout">{children}</div>
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

// Mock LocationBadge
vi.mock("@/components/ui/location-badge", () => ({
  LocationBadge: ({ student }: { student: { current_location: string } }) => (
    <span data-testid="location-badge">{student.current_location}</span>
  ),
}));

// Mock location helpers
vi.mock("~/lib/location-helper", () => ({
  isHomeLocation: (loc: string) => loc === "Zuhause" || loc === "",
  isPresentLocation: (loc: string) =>
    loc !== "Zuhause" &&
    loc !== "" &&
    loc !== "Unterwegs" &&
    loc !== "Schulhof",
  isTransitLocation: (loc: string) => loc === "Unterwegs",
  isSchoolyardLocation: (loc: string) => loc === "Schulhof",
}));

// Mock student-helpers
vi.mock("~/lib/student-helpers", () => ({
  SCHOOL_YEAR_FILTER_OPTIONS: [
    { value: "all", label: "Alle" },
    { value: "1", label: "1" },
    { value: "2", label: "2" },
    { value: "3", label: "3" },
    { value: "4", label: "4" },
  ],
}));

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
  },
  {
    id: "2",
    first_name: "Anna",
    second_name: "Schmidt",
    school_class: "2b",
    group_name: "Gruppe B",
    current_location: "Zuhause",
  },
  {
    id: "3",
    first_name: "Tom",
    second_name: "Weber",
    school_class: "1a",
    group_name: "Gruppe A",
    current_location: "Unterwegs",
  },
  {
    id: "4",
    first_name: "Lisa",
    second_name: "Müller",
    school_class: "3c",
    group_name: "Gruppe C",
    current_location: "Schulhof",
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
    vi.mocked(swrModule.useImmutableSWR).mockReturnValue({
      data: [
        { id: "1", name: "Gruppe A" },
        { id: "2", name: "Gruppe B" },
        { id: "3", name: "Gruppe C" },
      ],
      isLoading: false,
      error: null,
    } as ReturnType<typeof swrModule.useImmutableSWR>);
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
        error: null,
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
          "/students/1?from=/students/search",
        );
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
});
