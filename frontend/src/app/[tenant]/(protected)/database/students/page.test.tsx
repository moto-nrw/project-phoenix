import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import StudentsPage from "./page";
import { useSession } from "next-auth/react";

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { id: "1", token: "test-token" }, expires: "2099-01-01" },
    status: "authenticated",
  })),
}));

// Stateful URL search params mock. Tests mutate `currentSearch` via
// `setSelectedStudent(id)` before `render()` for scenarios that need
// the detail panel to show, and assert on `mockReplace` for click-driven
// navigations (which in the real page flow would update the URL).
let currentSearch: URLSearchParams = new URLSearchParams();
let suspendSearchParams = false;
const pendingSearchParams = new Promise<never>(() => undefined);
const mockReplace = vi.fn((url: string) => {
  const q = url.includes("?") ? (url.split("?")[1] ?? "") : "";
  currentSearch = new URLSearchParams(q);
});
const setSelectedStudent = (id: string | null) => {
  currentSearch = new URLSearchParams();
  if (id) currentSearch.set("student", id);
};

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: vi.fn(() => ({ push: vi.fn(), replace: mockReplace })),
  usePathname: vi.fn(() => "/tenant/database/students"),
  useSearchParams: () => {
    if (suspendSearchParams) throw pendingSearchParams;
    return currentSearch;
  },
}));

vi.mock("../section-skeleton", () => ({
  DatabaseSectionSkeleton: () => (
    <div data-testid="database-section-skeleton">Loading</div>
  ),
}));

// Mock SWR hooks
vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  mutate: vi.fn(),
  useTenantMutate: vi.fn(() => vi.fn()),
}));

// Mock service factory
const mockGetOne = vi.fn();
const mockCreate = vi.fn();
const mockUpdate = vi.fn();
const mockDelete = vi.fn();
vi.mock("@/lib/database/service-factory", () => ({
  createCrudService: vi.fn(() => ({
    getList: vi.fn(),
    getOne: mockGetOne,
    create: mockCreate,
    update: mockUpdate,
    delete: mockDelete,
  })),
}));

// Mock hooks
vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

const mockToastError = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: vi.fn(() => ({
    success: vi.fn(),
    error: mockToastError,
  })),
}));

// Mock UI components
vi.mock("~/components/database/database-page-layout", () => ({
  DatabasePageLayout: ({
    children,
    loading,
  }: {
    children: React.ReactNode;
    loading: boolean;
  }) => (
    <div data-testid="database-layout" data-loading={loading}>
      {children}
    </div>
  ),
}));

vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: ({
    search,
    filters,
    onClearAllFilters,
    actionButton,
  }: {
    search: { value: string; onChange: (v: string) => void };
    filters: Array<{
      id: string;
      value: string;
      onChange: (v: string) => void;
      options?: Array<{ value: string; label: string }>;
    }>;
    onClearAllFilters: () => void;
    actionButton?: ReactNode;
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
          {f.options?.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          )) ?? <option value="all">All</option>}
        </select>
      ))}
      <button
        type="button"
        data-testid="clear-filters"
        onClick={onClearAllFilters}
      >
        Clear
      </button>
      {actionButton}
    </div>
  ),
}));

// Test double for the master-detail layout. Exposes:
// - a clickable row per student (fires `onSelect(id)`)
// - guardian/group chips visible in the DOM for filter/display assertions
// - a "detail-panel" that renders when `selectedId` is set, with the
//   page-supplied `detailActions` plugged in AND a synthetic "trigger-update"
//   button that invokes the inline save path (formerly the edit modal).
vi.mock("@/components/students/students-master-detail", () => ({
  StudentsMasterDetail: ({
    students,
    selectedId,
    onSelect,
    onUpdateStudent,
    detailActions,
    canViewEnrollments,
  }: {
    students: Array<{
      id: string;
      first_name: string;
      second_name: string;
      name_lg?: string | null;
      group_name?: string | null;
    }>;
    selectedId: string | null;
    onSelect: (id: string | null) => void;
    onUpdateStudent: (
      id: string,
      data: { first_name: string; second_name: string },
    ) => Promise<void>;
    detailActions?: ReactNode;
    canViewEnrollments?: boolean;
  }) => (
    <div
      data-testid="students-master-detail"
      data-can-view-enrollments={String(Boolean(canViewEnrollments))}
    >
      {students.map((s) => (
        <div key={s.id} data-testid={`student-entry-${s.id}`}>
          <button
            type="button"
            data-testid={`student-row-${s.id}`}
            onClick={() => onSelect(String(s.id))}
          >
            {s.first_name} {s.second_name}
          </button>
          {s.name_lg ? (
            <span data-testid={`guardian-${s.id}`}>{s.name_lg}</span>
          ) : null}
          {s.group_name ? (
            <span data-testid={`group-${s.id}`}>{s.group_name}</span>
          ) : null}
        </div>
      ))}
      {selectedId ? (
        <div data-testid="student-detail-panel">
          <span data-testid="detail-selected-id">{selectedId}</span>
          <button
            type="button"
            data-testid="trigger-update"
            onClick={() =>
              void onUpdateStudent(selectedId, {
                first_name: "Updated",
                second_name: "Student",
              })
            }
          >
            Save
          </button>
          <button
            type="button"
            data-testid="trigger-deselect"
            onClick={() => onSelect(null)}
          >
            Close
          </button>
          <div data-testid="detail-actions">{detailActions}</div>
        </div>
      ) : null}
    </div>
  ),
}));

vi.mock("~/components/database/database-grouping-toggle", () => ({
  DatabaseGroupingToggle: () => <div data-testid="grouping-toggle" />,
}));

vi.mock("@/components/students/student-create-modal", () => ({
  StudentCreateModal: ({
    isOpen,
    onClose,
    onCreate,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onCreate: (data: {
      first_name: string;
      second_name: string;
      guardians?: Array<Record<string, unknown>>;
    }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="student-create-modal">
        <button
          type="button"
          data-testid="submit-create"
          onClick={() =>
            void onCreate({ first_name: "New", second_name: "Student" })
          }
        >
          Submit
        </button>
        <button
          type="button"
          data-testid="submit-create-with-guardians"
          onClick={() =>
            void onCreate({
              first_name: "New",
              second_name: "Student",
              guardians: [{ first_name: "Erika", relationship_type: "parent" }],
            })
          }
        >
          Submit with guardians
        </button>
        <button
          type="button"
          data-testid="close-create-modal"
          onClick={onClose}
        >
          Close
        </button>
      </div>
    ) : null,
}));

vi.mock("~/components/students/student-deletion-modal", () => ({
  StudentDeletionModal: ({
    isOpen,
    studentId,
    displayName,
    onDeleted,
    onClose,
  }: {
    isOpen: boolean;
    studentId: string;
    displayName: string;
    onDeleted?: () => void;
    onClose?: () => void;
  }) =>
    isOpen ? (
      <div
        data-testid="student-deletion-modal"
        data-student-id={studentId}
        data-display-name={displayName}
      >
        <button type="button" data-testid="finish-delete" onClick={onDeleted}>
          Confirm
        </button>
        <button type="button" data-testid="cancel-delete" onClick={onClose}>
          Cancel
        </button>
      </div>
    ) : null,
}));

// Import mocked modules
import { useSWRAuth } from "~/lib/swr";

function mockSessionWithPermissions(permissions: string[]) {
  vi.mocked(useSession).mockReturnValue({
    data: {
      user: { id: "1", token: "test-token", permissions },
      expires: "2099-01-01",
    },
    status: "authenticated",
  } as ReturnType<typeof useSession>);
}

const mockStudents = [
  {
    id: "1",
    first_name: "Max",
    second_name: "Mustermann",
    school_class: "1a",
    group_id: "g1",
    group_name: "Gruppe A",
    name_lg: "Hans Mustermann",
  },
  {
    id: "2",
    first_name: "Anna",
    second_name: "Schmidt",
    school_class: "2b",
    group_id: "g2",
    group_name: "Gruppe B",
    name_lg: null,
  },
  // Sparse fixture: all string fields except name_lg are nullish so the
  // optional-chaining branches in the search filter (`student.first_name?.…`,
  // `student.second_name?.…`, `student.school_class?.…`, `student.group_name?.…`)
  // exercise their falsy paths when a search runs.
  {
    id: "3",
    first_name: null,
    second_name: null,
    school_class: null,
    group_id: null,
    group_name: null,
    name_lg: "Erika Beispiel",
  },
];

describe("StudentsPage", () => {
  beforeEach(() => {
    suspendSearchParams = false;
    vi.clearAllMocks();
    setSelectedStudent(null);
    mockSessionWithPermissions(["config:manage", "users:delete"]);

    // Default SWR mock - returns students data
    vi.mocked(useSWRAuth).mockImplementation((key: string | null) => {
      if (key === "database-students-list") {
        return {
          data: mockStudents,
          isLoading: false,
          error: null,
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      }
      // Groups dropdown
      return {
        data: [
          { value: "g1", label: "Gruppe A" },
          { value: "g2", label: "Gruppe B" },
        ],
        isLoading: false,
        error: null,
        isValidating: false,
        mutate: vi.fn(),
      } as ReturnType<typeof useSWRAuth>;
    });

    // Setup getOne to return the selected student
    mockGetOne.mockImplementation((id: string) =>
      Promise.resolve(mockStudents.find((s) => s.id === id)),
    );
  });

  it("shows the database skeleton while the route suspends", () => {
    suspendSearchParams = true;

    render(<StudentsPage />);

    expect(screen.getByTestId("database-section-skeleton")).toBeVisible();
  });

  it("renders the page with students data", async () => {
    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
      expect(screen.getByText("Anna Schmidt")).toBeInTheDocument();
    });
  });

  it("does not expose enrollment tab access for config read-only users", async () => {
    mockSessionWithPermissions(["config:read"]);

    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("students-master-detail")).toHaveAttribute(
        "data-can-view-enrollments",
        "false",
      );
    });
  });

  it("exposes enrollment tab access for config manage users", async () => {
    mockSessionWithPermissions(["config:manage"]);

    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("students-master-detail")).toHaveAttribute(
        "data-can-view-enrollments",
        "true",
      );
    });
  });

  it("does not expose the deletion workflow without users delete permission", async () => {
    mockSessionWithPermissions(["config:manage"]);
    setSelectedStudent("1");

    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-detail-panel")).toBeInTheDocument();
    });
    expect(
      screen.queryByRole("button", { name: /Löschen/i }),
    ).not.toBeInTheDocument();
  });

  it("shows loading state when data is loading", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<StudentsPage />);

    const layout = screen.getByTestId("database-layout");
    expect(layout).toHaveAttribute("data-loading", "true");
  });

  it("shows error message when fetch fails", async () => {
    vi.mocked(useSWRAuth).mockImplementation((key: string | null) => {
      if (key === "database-students-list") {
        return {
          data: undefined,
          isLoading: false,
          error: new Error("Failed to fetch"),
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      }
      return {
        data: [],
        isLoading: false,
        error: null,
        isValidating: false,
        mutate: vi.fn(),
      } as ReturnType<typeof useSWRAuth>;
    });

    render(<StudentsPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Kinder/),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state when no students exist", async () => {
    vi.mocked(useSWRAuth).mockImplementation((key: string | null) => {
      if (key === "database-students-list") {
        return {
          data: [],
          isLoading: false,
          error: null,
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      }
      return {
        data: [],
        isLoading: false,
        error: null,
        isValidating: false,
        mutate: vi.fn(),
      } as ReturnType<typeof useSWRAuth>;
    });

    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Kinder vorhanden")).toBeInTheDocument();
    });
  });

  it("filters students by search term", async () => {
    render(<StudentsPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Max" } });

    await waitFor(() => {
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
      expect(screen.queryByText("Anna Schmidt")).not.toBeInTheDocument();
    });
  });

  it("filters students by full name across first and last name", async () => {
    render(<StudentsPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Max Mu" } });

    await waitFor(() => {
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
      expect(screen.queryByText("Anna Schmidt")).not.toBeInTheDocument();
    });
  });

  it("filters students by last name", async () => {
    render(<StudentsPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Muster" } });

    await waitFor(() => {
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
      expect(screen.queryByText("Anna Schmidt")).not.toBeInTheDocument();
    });
  });

  it("filters students by reversed full name", async () => {
    render(<StudentsPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Mustermann Max" } });

    await waitFor(() => {
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
      expect(screen.queryByText("Anna Schmidt")).not.toBeInTheDocument();
    });
  });

  it("filters students by guardian name (name_lg)", async () => {
    render(<StudentsPage />);

    // name_lg is "Hans Mustermann" for student with id "1"
    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Hans" } });

    await waitFor(() => {
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
      // Anna has null name_lg, so should be filtered out
      expect(screen.queryByText("Anna Schmidt")).not.toBeInTheDocument();
    });
  });

  it("matches a student whose name and class fields are nullish", async () => {
    render(<StudentsPage />);

    // Student id "3" has first_name/second_name/school_class/group_name all null;
    // searching by guardian name still matches and exercises every `?.` falsy
    // branch in the search filter.
    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Erika" } });

    await waitFor(() => {
      expect(screen.getByText(/Erika Beispiel/)).toBeInTheDocument();
      expect(screen.queryByText("Max Mustermann")).not.toBeInTheDocument();
      expect(screen.queryByText("Anna Schmidt")).not.toBeInTheDocument();
    });
  });

  it("filters students by school class", async () => {
    render(<StudentsPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "1a" } });

    await waitFor(() => {
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
      // Anna is in 2b, so should be filtered out
      expect(screen.queryByText("Anna Schmidt")).not.toBeInTheDocument();
    });
  });

  it("filters students by group name", async () => {
    render(<StudentsPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Gruppe A" } });

    await waitFor(() => {
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
      expect(screen.queryByText("Anna Schmidt")).not.toBeInTheDocument();
    });
  });

  it("clears all filters when clear button is clicked", async () => {
    render(<StudentsPage />);

    // Set a search term first
    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "test" } });

    // Click clear filters
    const clearButton = screen.getByTestId("clear-filters");
    fireEvent.click(clearButton);

    await waitFor(() => {
      expect(searchInput).toHaveValue("");
    });
  });

  it("displays group badges for students with groups", async () => {
    render(<StudentsPage />);

    await waitFor(() => {
      // Use getAllByText since "Gruppe A/B" appears in both dropdown and badges
      expect(screen.getAllByText("Gruppe A").length).toBeGreaterThan(0);
      expect(screen.getAllByText("Gruppe B").length).toBeGreaterThan(0);
    });
  });

  it("displays guardian info when available", async () => {
    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByText(/Hans Mustermann/)).toBeInTheDocument();
    });
  });

  it("opens create modal when create button is clicked", async () => {
    render(<StudentsPage />);

    const createButton = screen.getAllByLabelText("Kind erstellen")[0]!;
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByTestId("student-create-modal")).toBeInTheDocument();
    });
  });

  it("navigates to the selected student when a row is clicked", async () => {
    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("student-row-1"));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalled();
    });
    const lastCallArg = mockReplace.mock.calls.at(-1)?.[0] ?? "";
    expect(lastCallArg).toContain("student=1");
  });

  it("clears the selection when the detail close action is triggered", async () => {
    setSelectedStudent("1");
    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-deselect"));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalled();
    });
    const lastCallArg = mockReplace.mock.calls.at(-1)?.[0] ?? "";
    expect(lastCallArg).not.toContain("student=");
  });

  it("renders the detail panel when a student is selected via URL", async () => {
    setSelectedStudent("1");
    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-detail-panel")).toBeInTheDocument();
      expect(screen.getByTestId("detail-selected-id")).toHaveTextContent("1");
    });
  });

  it("calls create service when submitting create form", async () => {
    mockCreate.mockResolvedValueOnce({
      id: "3",
      first_name: "New",
      second_name: "Student",
    });

    render(<StudentsPage />);

    // Open create modal
    const createButton = screen.getAllByLabelText("Kind erstellen")[0]!;
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByTestId("student-create-modal")).toBeInTheDocument();
    });

    // Submit the form
    const submitButton = screen.getByTestId("submit-create");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalled();
    });
  });

  it("forwards guardians to the create service when present", async () => {
    mockCreate.mockResolvedValueOnce({
      id: "4",
      first_name: "New",
      second_name: "Student",
    });

    render(<StudentsPage />);

    const createButton = screen.getAllByLabelText("Kind erstellen")[0]!;
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByTestId("student-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create-with-guardians"));

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalled();
    });
    // The guardians must survive transformBeforeSubmit and reach service.create.
    const payload = mockCreate.mock.calls[0]?.[0] as Record<string, unknown>;
    expect(payload.guardians).toEqual([
      { first_name: "Erika", relationship_type: "parent" },
    ]);
  });

  it("calls update service when the detail panel saves Stammdaten", async () => {
    mockUpdate.mockResolvedValueOnce({
      id: "1",
      first_name: "Updated",
      second_name: "Student",
    });

    setSelectedStudent("1");
    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-update"));

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalled();
    });
    expect(mockUpdate.mock.calls[0]?.[0]).toBe("1");
  });

  it("opens the guarded deletion workflow for the selected student", async () => {
    setSelectedStudent("1");
    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /Löschen/i }));

    await waitFor(() => {
      expect(screen.getByTestId("student-deletion-modal")).toHaveAttribute(
        "data-student-id",
        "1",
      );
    });
    expect(screen.getByTestId("student-deletion-modal")).toHaveAttribute(
      "data-display-name",
      "Max Mustermann",
    );
  });

  it("clears the selected child after the guarded workflow succeeds", async () => {
    setSelectedStudent("1");
    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-detail-panel")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: /Löschen/i }));
    fireEvent.click(await screen.findByTestId("finish-delete"));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalled();
    });
    expect(
      screen.queryByTestId("student-deletion-modal"),
    ).not.toBeInTheDocument();
  });

  it("shows not found message when search has no matches", async () => {
    render(<StudentsPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "xyz123" } });

    await waitFor(() => {
      expect(screen.getByText("Keine Kinder gefunden")).toBeInTheDocument();
    });
  });

  // Tests for SWR fetcher execution (new code coverage)
  describe("SWR fetcher execution", () => {
    it("executes the students SWR fetcher and handles array response", async () => {
      const mockGetList = vi.fn().mockResolvedValue({
        data: [{ id: "1", first_name: "Test", second_name: "Student" }],
      });

      const serviceFactory = await import("@/lib/database/service-factory");
      vi.mocked(serviceFactory.createCrudService).mockReturnValue({
        getList: mockGetList,
        getOne: mockGetOne,
        create: mockCreate,
        update: mockUpdate,
        delete: mockDelete,
      });

      let capturedStudentsFetcher: (() => Promise<unknown>) | null = null;

      vi.mocked(useSWRAuth).mockImplementation((key, fetcher) => {
        if (key === "database-students-list" && fetcher) {
          capturedStudentsFetcher = fetcher as () => Promise<unknown>;
        }
        if (key === "database-students-list") {
          return {
            data: [
              {
                id: "1",
                first_name: "Test",
                second_name: "Student",
                school_class: "1a",
              },
            ],
            isLoading: false,
            error: null,
            isValidating: false,
            mutate: vi.fn(),
          } as ReturnType<typeof useSWRAuth>;
        }
        return {
          data: [],
          isLoading: false,
          error: null,
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      });

      render(<StudentsPage />);

      // Execute the captured fetcher to cover lines 75-78
      expect(capturedStudentsFetcher).not.toBeNull();
      const result: unknown = await (
        capturedStudentsFetcher as unknown as () => Promise<unknown>
      )();
      expect(result).toEqual([
        { id: "1", first_name: "Test", second_name: "Student" },
      ]);
      expect(mockGetList).toHaveBeenCalledWith({
        page: 1,
        pageSize: 1000,
        include_arrival_times: true,
      });
    });

    it("handles non-array response from getList", async () => {
      const mockGetList = vi.fn().mockResolvedValue({
        data: "not an array",
      });

      const serviceFactory = await import("@/lib/database/service-factory");
      vi.mocked(serviceFactory.createCrudService).mockReturnValue({
        getList: mockGetList,
        getOne: mockGetOne,
        create: mockCreate,
        update: mockUpdate,
        delete: mockDelete,
      });

      let capturedStudentsFetcher: (() => Promise<unknown>) | null = null;

      vi.mocked(useSWRAuth).mockImplementation((key, fetcher) => {
        if (key === "database-students-list" && fetcher) {
          capturedStudentsFetcher = fetcher as () => Promise<unknown>;
        }
        return {
          data: [],
          isLoading: false,
          error: null,
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      });

      render(<StudentsPage />);

      expect(capturedStudentsFetcher).not.toBeNull();
      const result: unknown = await (
        capturedStudentsFetcher as unknown as () => Promise<unknown>
      )();
      // Should return empty array when data is not an array
      expect(result).toEqual([]);
    });

    it("executes the groups dropdown SWR fetcher with array response", async () => {
      let capturedGroupsFetcher: (() => Promise<unknown>) | null = null;

      // Mock fetch for groups API
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve([
            { id: 1, name: "Group Alpha" },
            { id: 2, name: "Group Beta" },
          ]),
      });
      vi.stubGlobal("fetch", mockFetch);

      vi.mocked(useSWRAuth).mockImplementation((key, fetcher) => {
        if (key === "database-groups-dropdown" && fetcher) {
          capturedGroupsFetcher = fetcher as () => Promise<unknown>;
        }
        if (key === "database-students-list") {
          return {
            data: mockStudents,
            isLoading: false,
            error: null,
            isValidating: false,
            mutate: vi.fn(),
          } as ReturnType<typeof useSWRAuth>;
        }
        return {
          data: [],
          isLoading: false,
          error: null,
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      });

      render(<StudentsPage />);

      expect(capturedGroupsFetcher).not.toBeNull();
      const result: unknown = await (
        capturedGroupsFetcher as unknown as () => Promise<unknown>
      )();
      expect(result).toEqual([
        { value: "1", label: "Group Alpha" },
        { value: "2", label: "Group Beta" },
      ]);

      vi.unstubAllGlobals();
    });

    it("handles wrapped groups response with data property", async () => {
      let capturedGroupsFetcher: (() => Promise<unknown>) | null = null;

      // Mock fetch with wrapped response { data: [...] }
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            data: [
              { id: 3, name: "Wrapped Group A" },
              { id: 4, name: "Wrapped Group B" },
            ],
          }),
      });
      vi.stubGlobal("fetch", mockFetch);

      vi.mocked(useSWRAuth).mockImplementation((key, fetcher) => {
        if (key === "database-groups-dropdown" && fetcher) {
          capturedGroupsFetcher = fetcher as () => Promise<unknown>;
        }
        if (key === "database-students-list") {
          return {
            data: mockStudents,
            isLoading: false,
            error: null,
            isValidating: false,
            mutate: vi.fn(),
          } as ReturnType<typeof useSWRAuth>;
        }
        return {
          data: [],
          isLoading: false,
          error: null,
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      });

      render(<StudentsPage />);

      expect(capturedGroupsFetcher).not.toBeNull();
      const result: unknown = await (
        capturedGroupsFetcher as unknown as () => Promise<unknown>
      )();
      expect(result).toEqual([
        { value: "3", label: "Wrapped Group A" },
        { value: "4", label: "Wrapped Group B" },
      ]);

      vi.unstubAllGlobals();
    });

    it("handles failed fetch for groups", async () => {
      let capturedGroupsFetcher: (() => Promise<unknown>) | null = null;

      // Mock fetch with error response
      const mockFetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
      });
      vi.stubGlobal("fetch", mockFetch);

      // Spy on console.error
      const consoleErrorSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => undefined);

      vi.mocked(useSWRAuth).mockImplementation((key, fetcher) => {
        if (key === "database-groups-dropdown" && fetcher) {
          capturedGroupsFetcher = fetcher as () => Promise<unknown>;
        }
        if (key === "database-students-list") {
          return {
            data: mockStudents,
            isLoading: false,
            error: null,
            isValidating: false,
            mutate: vi.fn(),
          } as ReturnType<typeof useSWRAuth>;
        }
        return {
          data: [],
          isLoading: false,
          error: null,
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      });

      render(<StudentsPage />);

      expect(capturedGroupsFetcher).not.toBeNull();
      const result: unknown = await (
        capturedGroupsFetcher as unknown as () => Promise<unknown>
      )();
      expect(result).toEqual([]);
      expect(consoleErrorSpy).toHaveBeenCalledWith("failed to fetch groups", {
        status: 500,
      });

      consoleErrorSpy.mockRestore();
      vi.unstubAllGlobals();
    });

    it("handles unexpected groups response format", async () => {
      let capturedGroupsFetcher: (() => Promise<unknown>) | null = null;

      // Mock fetch with unexpected format
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve("unexpected string"),
      });
      vi.stubGlobal("fetch", mockFetch);

      const consoleErrorSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => undefined);

      vi.mocked(useSWRAuth).mockImplementation((key, fetcher) => {
        if (key === "database-groups-dropdown" && fetcher) {
          capturedGroupsFetcher = fetcher as () => Promise<unknown>;
        }
        if (key === "database-students-list") {
          return {
            data: mockStudents,
            isLoading: false,
            error: null,
            isValidating: false,
            mutate: vi.fn(),
          } as ReturnType<typeof useSWRAuth>;
        }
        return {
          data: [],
          isLoading: false,
          error: null,
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      });

      render(<StudentsPage />);

      expect(capturedGroupsFetcher).not.toBeNull();
      const result: unknown = await (
        capturedGroupsFetcher as unknown as () => Promise<unknown>
      )();
      expect(result).toEqual([]);
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "unexpected groups response format",
        { data_type: "string" },
      );

      consoleErrorSpy.mockRestore();
      vi.unstubAllGlobals();
    });
  });

  // Tests for group filter coverage (line 138-140)
  describe("Group Filter", () => {
    it("filters students by group selection", async () => {
      const studentsWithGroups = [
        {
          id: "1",
          first_name: "Max",
          second_name: "Mustermann",
          school_class: "1a",
          group_id: "group-1",
          group_name: "Gruppe A",
        },
        {
          id: "2",
          first_name: "Anna",
          second_name: "Schmidt",
          school_class: "2b",
          group_id: "group-2",
          group_name: "Gruppe B",
        },
      ];

      // Mock useSWRAuth to return different data based on cache key
      vi.mocked(useSWRAuth).mockImplementation((key) => {
        if (key === "database-students-list") {
          return {
            data: studentsWithGroups,
            isLoading: false,
            error: null,
            isValidating: false,
            mutate: vi.fn(),
          } as ReturnType<typeof useSWRAuth>;
        }
        // For groups dropdown
        return {
          data: [
            { value: "group-1", label: "Gruppe A" },
            { value: "group-2", label: "Gruppe B" },
          ],
          isLoading: false,
          error: null,
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      });

      render(<StudentsPage />);

      await waitFor(() => {
        expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
        expect(screen.getByText("Anna Schmidt")).toBeInTheDocument();
      });

      // Change group filter to "group-1"
      const groupFilter = screen.getByTestId("filter-group");
      fireEvent.change(groupFilter, { target: { value: "group-1" } });

      await waitFor(() => {
        // Max (group-1) should be visible
        expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
        // Anna (group-2) should be filtered out
        expect(screen.queryByText("Anna Schmidt")).not.toBeInTheDocument();
      });
    });

    it("shows empty state when group filter has no matches", async () => {
      // Mock useSWRAuth to return different data based on cache key
      vi.mocked(useSWRAuth).mockImplementation((key) => {
        if (key === "database-students-list") {
          return {
            data: [
              {
                id: "1",
                first_name: "Max",
                second_name: "Mustermann",
                school_class: "1a",
                group_id: "group-1",
                group_name: "Gruppe A",
              },
            ],
            isLoading: false,
            error: null,
            isValidating: false,
            mutate: vi.fn(),
          } as ReturnType<typeof useSWRAuth>;
        }
        // For groups dropdown
        return {
          data: [{ value: "group-1", label: "Gruppe A" }],
          isLoading: false,
          error: null,
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      });

      render(<StudentsPage />);

      await waitFor(() => {
        expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
      });

      // Change group filter to a non-existing group
      const groupFilter = screen.getByTestId("filter-group");
      fireEvent.change(groupFilter, {
        target: { value: "non-existing-group" },
      });

      await waitFor(() => {
        expect(screen.getByText("Keine Kinder gefunden")).toBeInTheDocument();
      });
    });
  });
});
