import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import TeachersPage from "./page";

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { id: "1", token: "test-token" }, expires: "2099-01-01" },
    status: "authenticated",
  })),
}));

// Mock next/navigation
vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: vi.fn(() => ({ push: vi.fn() })),
}));

// Mock SWR hooks
vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  mutate: vi.fn(),
}));

// Mock service factory
vi.mock("@/lib/database/service-factory", () => ({
  createCrudService: vi.fn(() => ({
    getList: vi.fn(),
    getOne: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
  })),
}));

// Mock hooks
vi.mock("~/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

vi.mock("~/hooks/useDeleteConfirmation", () => ({
  useDeleteConfirmation: vi.fn(() => ({
    showConfirmModal: false,
    handleDeleteClick: vi.fn(),
    handleDeleteCancel: vi.fn(),
    confirmDelete: vi.fn(),
  })),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: vi.fn(() => ({
    success: vi.fn(),
    error: vi.fn(),
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

vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({
    search,
    onClearAllFilters,
  }: {
    search: { value: string; onChange: (v: string) => void };
    onClearAllFilters: () => void;
  }) => (
    <div data-testid="page-header">
      <input
        data-testid="search-input"
        value={search.value}
        onChange={(e) => search.onChange(e.target.value)}
      />
      <button data-testid="clear-filters" onClick={onClearAllFilters}>
        Clear
      </button>
    </div>
  ),
}));

vi.mock("@/components/teachers", () => ({
  TeacherRoleManagementModal: () => <div data-testid="role-modal" />,
  TeacherPermissionManagementModal: () => (
    <div data-testid="permission-modal" />
  ),
}));

vi.mock("@/components/teachers/teacher-detail-modal", () => ({
  TeacherDetailModal: () => <div data-testid="teacher-detail-modal" />,
}));

vi.mock("@/components/teachers/teacher-edit-modal", () => ({
  TeacherEditModal: () => <div data-testid="teacher-edit-modal" />,
}));

vi.mock("@/components/teachers/teacher-create-modal", () => ({
  TeacherCreateModal: () => <div data-testid="teacher-create-modal" />,
}));

vi.mock("~/components/ui/modal", () => ({
  Modal: () => <div data-testid="modal" />,
  ConfirmationModal: () => <div data-testid="confirmation-modal" />,
}));

// Import mocked modules
import { useSWRAuth } from "~/lib/swr";

const mockTeachers = [
  {
    id: "1",
    first_name: "Maria",
    last_name: "Müller",
    email: "maria@example.com",
    roles: ["teacher"],
  },
  {
    id: "2",
    first_name: "Thomas",
    last_name: "Schmidt",
    email: "thomas@example.com",
    roles: ["admin", "teacher"],
  },
];

describe("TeachersPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockTeachers,
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);
  });

  it("renders the page with teachers data", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
      expect(screen.getByText("Thomas Schmidt")).toBeInTheDocument();
    });
  });

  it("shows loading state when data is loading", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<TeachersPage />);

    const layout = screen.getByTestId("database-layout");
    expect(layout).toHaveAttribute("data-loading", "true");
  });

  it("shows error message when fetch fails", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error("Failed to fetch"),
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<TeachersPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Betreuer/),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state when no teachers exist", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Betreuer vorhanden")).toBeInTheDocument();
    });
  });

  it("filters teachers by search term", async () => {
    render(<TeachersPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Maria" } });

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
      expect(screen.queryByText("Thomas Schmidt")).not.toBeInTheDocument();
    });
  });

  it("displays email for teachers", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("maria@example.com")).toBeInTheDocument();
    });
  });
});
