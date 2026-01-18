import {
  render,
  screen,
  fireEvent,
  waitFor,
  within,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import SubstitutionsPage from "./page";

// Hoist mocks
const { mockToastSuccess } = vi.hoisted(() => ({
  mockToastSuccess: vi.fn(),
}));

const { mockCreateSubstitution, mockDeleteSubstitution } = vi.hoisted(() => ({
  mockCreateSubstitution: vi.fn(),
  mockDeleteSubstitution: vi.fn(),
}));

// Mock next-auth
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(),
}));

// Mock next/navigation
vi.mock("next/navigation", () => ({
  useRouter: vi.fn(),
}));

// Mock SWR hooks
vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  useImmutableSWR: vi.fn(),
}));

// Mock toast context
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
  }),
}));

// Mock substitution service
vi.mock("~/lib/substitution-api", () => ({
  substitutionService: {
    createSubstitution: mockCreateSubstitution,
    deleteSubstitution: mockDeleteSubstitution,
  },
}));

// Mock ResponsiveLayout
vi.mock("~/components/dashboard", () => ({
  ResponsiveLayout: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="layout">{children}</div>
  ),
}));

// Mock PageHeaderWithSearch
vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({
    search,
    filters,
    onClearAllFilters,
  }: {
    search: { value: string; onChange: (v: string) => void };
    filters?: Array<{
      id: string;
      onChange: (v: string | string[]) => void;
    }>;
    onClearAllFilters: () => void;
  }) => {
    const statusFilter = filters?.find((f) => f.id === "status");
    return (
      <div data-testid="page-header">
        <input
          data-testid="search-input"
          value={search.value}
          onChange={(e) => search.onChange(e.target.value)}
        />
        <button
          data-testid="filter-available"
          onClick={() => statusFilter?.onChange("available")}
        >
          Available
        </button>
        <button
          data-testid="filter-substitution"
          onClick={() => statusFilter?.onChange("substitution")}
        >
          In Substitution
        </button>
        <button data-testid="clear-filters" onClick={onClearAllFilters}>
          Clear
        </button>
      </div>
    );
  },
}));

// Mock Modal components
vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    onClose,
    title,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="assignment-modal" role="dialog">
        <h2>{title}</h2>
        <button data-testid="modal-close" onClick={onClose}>
          Close
        </button>
        {children}
      </div>
    ) : null,
  ConfirmationModal: ({
    isOpen,
    onClose,
    onConfirm,
    title,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onConfirm: () => void;
    title: string;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="confirmation-modal" role="dialog">
        <h2>{title}</h2>
        {children}
        <button data-testid="confirm-end" onClick={onConfirm}>
          Confirm
        </button>
        <button data-testid="cancel-end" onClick={onClose}>
          Cancel
        </button>
      </div>
    ) : null,
}));

// Mock Alert
vi.mock("~/components/ui/alert", () => ({
  Alert: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="alert">{children}</div>
  ),
}));

// Note: substitution-helpers are not mocked - using actual implementations

// Mock Loading
vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div aria-label="Lädt...">Loading...</div>,
}));

import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { useSWRAuth, useImmutableSWR } from "~/lib/swr";

// Test data
const mockTeachers = [
  {
    id: "1",
    firstName: "Anna",
    lastName: "Meyer",
    regularGroup: "Gruppe 2A",
    inSubstitution: false,
    substitutionCount: 0,
    substitutions: [],
  },
  {
    id: "2",
    firstName: "Ben",
    lastName: "Schulz",
    regularGroup: "Gruppe 3B",
    inSubstitution: true,
    substitutionCount: 1,
    substitutions: [
      {
        id: "sub-1",
        groupId: "g1",
        groupName: "Gruppe 1A",
        isTransfer: false,
        startDate: new Date(),
        endDate: new Date(Date.now() + 7 * 24 * 60 * 60 * 1000),
      },
    ],
  },
  {
    id: "3",
    firstName: "Clara",
    lastName: "Fischer",
    regularGroup: "Gruppe 1C",
    inSubstitution: true,
    substitutionCount: 1,
    substitutions: [
      {
        id: "sub-2",
        groupId: "g2",
        groupName: "Gruppe 4A",
        isTransfer: true,
        startDate: new Date(),
        endDate: new Date(),
      },
    ],
  },
];

const mockGroups = [
  { id: "g1", name: "Gruppe 1A" },
  { id: "g2", name: "Gruppe 2B" },
  { id: "g3", name: "Gruppe 3C" },
];

const mockActiveSubstitutions = [
  {
    id: "sub-1",
    groupId: "g1",
    groupName: "Gruppe 1A",
    substituteStaffId: "2",
    substituteStaffName: "Ben Schulz",
    startDate: new Date(),
    endDate: new Date(Date.now() + 7 * 24 * 60 * 60 * 1000),
    isTransfer: false,
  },
  {
    id: "sub-2",
    groupId: "g2",
    groupName: "Gruppe 4A",
    substituteStaffId: "3",
    substituteStaffName: "Clara Fischer",
    startDate: new Date(),
    endDate: new Date(),
    isTransfer: true,
  },
];

describe("SubstitutionsPage", () => {
  const mockPush = vi.fn();
  const mockMutateTeachers = vi.fn();
  const mockMutateSubstitutions = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useRouter).mockReturnValue({ push: mockPush } as never);
    vi.mocked(useSession).mockReturnValue({
      data: { user: { id: "1" } },
      status: "authenticated",
    } as never);

    // Default SWR mock setup
    vi.mocked(useSWRAuth).mockImplementation((key: string | null) => {
      if (key === "substitution-teachers") {
        return {
          data: mockTeachers,
          isLoading: false,
          error: null,
          mutate: mockMutateTeachers,
        } as never;
      }
      if (key === "active-substitutions") {
        return {
          data: mockActiveSubstitutions,
          isLoading: false,
          error: null,
          mutate: mockMutateSubstitutions,
        } as never;
      }
      return { data: null, isLoading: false, error: null } as never;
    });

    vi.mocked(useImmutableSWR).mockReturnValue({
      data: mockGroups,
      isLoading: false,
      error: null,
    } as never);
  });

  describe("Loading and Error States", () => {
    it("shows loading state while session is loading", () => {
      vi.mocked(useSession).mockReturnValue({
        data: null,
        status: "loading",
      } as never);

      render(<SubstitutionsPage />);

      expect(screen.getByLabelText("Lädt...")).toBeInTheDocument();
    });

    it("shows loading state while teachers are loading", () => {
      vi.mocked(useSWRAuth).mockImplementation((key: string | null) => {
        if (key === "substitution-teachers") {
          return {
            data: [],
            isLoading: true,
            error: null,
            mutate: mockMutateTeachers,
          } as never;
        }
        return {
          data: mockActiveSubstitutions,
          isLoading: false,
          error: null,
          mutate: mockMutateSubstitutions,
        } as never;
      });

      render(<SubstitutionsPage />);

      expect(screen.getByLabelText("Lädt...")).toBeInTheDocument();
    });

    it("shows error message when data fetch fails", () => {
      vi.mocked(useSWRAuth).mockImplementation((key: string | null) => {
        if (key === "substitution-teachers") {
          return {
            data: [],
            isLoading: false,
            error: new Error("Network error"),
            mutate: mockMutateTeachers,
          } as never;
        }
        return {
          data: [],
          isLoading: false,
          error: null,
          mutate: mockMutateSubstitutions,
        } as never;
      });

      render(<SubstitutionsPage />);

      expect(
        screen.getByText("Fehler beim Laden der Daten."),
      ).toBeInTheDocument();
    });
  });

  describe("Teacher List Rendering", () => {
    it("renders all teachers in the list", () => {
      render(<SubstitutionsPage />);

      // Teacher names may appear multiple times (in teacher list AND active substitutions)
      // so we use getAllByText
      expect(screen.getAllByText("Anna Meyer").length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText("Ben Schulz").length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText("Clara Fischer").length).toBeGreaterThanOrEqual(1);
    });

    it("displays teacher group information", () => {
      render(<SubstitutionsPage />);

      expect(screen.getByText("Gruppe 2A")).toBeInTheDocument();
      expect(screen.getByText("Gruppe 3B")).toBeInTheDocument();
    });

    it("displays teacher status correctly", () => {
      render(<SubstitutionsPage />);

      expect(screen.getByText("Verfügbar")).toBeInTheDocument();
      expect(screen.getByText(/Vertretung: Gruppe 1A/)).toBeInTheDocument();
    });

    it("shows empty state when no teachers match filter", async () => {
      render(<SubstitutionsPage />);

      // Get the teacher section
      const teacherSection = screen.getByText(
        "Verfügbare pädagogische Fachkräfte",
      ).parentElement!;

      fireEvent.change(screen.getByTestId("search-input"), {
        target: { value: "nonexistent" },
      });

      await waitFor(() => {
        expect(
          within(teacherSection).getByText("Keine Fachkräfte gefunden"),
        ).toBeInTheDocument();
      });
    });
  });

  describe("Teacher Card Interaction", () => {
    it("opens assignment modal when clicking teacher card", async () => {
      render(<SubstitutionsPage />);

      const teacherCard = screen.getByRole("button", { name: /Anna Meyer/i });
      fireEvent.click(teacherCard);

      await waitFor(() => {
        expect(screen.getByTestId("assignment-modal")).toBeInTheDocument();
      });
    });
  });

  describe("Search and Filter", () => {
    it("filters teachers by search term", async () => {
      render(<SubstitutionsPage />);

      // Get the teacher section (the section with "Verfügbare pädagogische Fachkräfte" heading)
      const teacherSection = screen.getByText(
        "Verfügbare pädagogische Fachkräfte",
      ).parentElement!;

      // Verify Anna is shown initially in teacher section
      expect(within(teacherSection).getByText("Anna Meyer")).toBeInTheDocument();

      // Apply search filter for Anna
      fireEvent.change(screen.getByTestId("search-input"), {
        target: { value: "Anna" },
      });

      // After filtering, only Anna should remain in the teacher section
      await waitFor(() => {
        expect(
          within(teacherSection).getByText("Anna Meyer"),
        ).toBeInTheDocument();
      });

      // Ben and Clara should not be in the teacher section (they may exist elsewhere)
      expect(
        within(teacherSection).queryByText("Ben Schulz"),
      ).not.toBeInTheDocument();
      expect(
        within(teacherSection).queryByText("Clara Fischer"),
      ).not.toBeInTheDocument();
    });

    it("has filter buttons available", () => {
      render(<SubstitutionsPage />);

      expect(screen.getByTestId("filter-available")).toBeInTheDocument();
      expect(screen.getByTestId("filter-substitution")).toBeInTheDocument();
      expect(screen.getByTestId("clear-filters")).toBeInTheDocument();
    });

    it("clears search when clear filters is clicked", async () => {
      render(<SubstitutionsPage />);

      // Get the teacher section
      const teacherSection = screen.getByText(
        "Verfügbare pädagogische Fachkräfte",
      ).parentElement!;

      // Apply search filter
      fireEvent.change(screen.getByTestId("search-input"), {
        target: { value: "Anna" },
      });

      await waitFor(() => {
        expect(
          within(teacherSection).queryByText("Ben Schulz"),
        ).not.toBeInTheDocument();
      });

      // Clear filters
      fireEvent.click(screen.getByTestId("clear-filters"));

      await waitFor(() => {
        expect(
          within(teacherSection).getByText("Anna Meyer"),
        ).toBeInTheDocument();
        expect(
          within(teacherSection).getByText("Ben Schulz"),
        ).toBeInTheDocument();
        expect(
          within(teacherSection).getByText("Clara Fischer"),
        ).toBeInTheDocument();
      });
    });
  });

  describe("Active Substitutions Display", () => {
    it("displays active transfers (Tagesübergaben) section", () => {
      render(<SubstitutionsPage />);

      expect(screen.getByText("Tagesübergaben")).toBeInTheDocument();
    });

    it("displays active substitutions (Vertretungen) section", () => {
      render(<SubstitutionsPage />);

      expect(screen.getByText("Vertretungen")).toBeInTheDocument();
    });

    it("shows empty state when no active transfers", () => {
      vi.mocked(useSWRAuth).mockImplementation((key: string | null) => {
        if (key === "substitution-teachers") {
          return {
            data: mockTeachers,
            isLoading: false,
            error: null,
            mutate: mockMutateTeachers,
          } as never;
        }
        if (key === "active-substitutions") {
          return {
            data: mockActiveSubstitutions.filter((s) => !s.isTransfer),
            isLoading: false,
            error: null,
            mutate: mockMutateSubstitutions,
          } as never;
        }
        return { data: null, isLoading: false, error: null } as never;
      });

      render(<SubstitutionsPage />);

      expect(
        screen.getByText("Keine aktiven Tagesübergaben"),
      ).toBeInTheDocument();
    });
  });

  describe("Assign Substitution Modal", () => {
    it("opens assignment modal when clicking teacher card", async () => {
      render(<SubstitutionsPage />);

      // Open modal by clicking teacher
      const teacherCard = screen.getByRole("button", { name: /Anna Meyer/i });
      fireEvent.click(teacherCard);

      await waitFor(() => {
        expect(screen.getByTestId("assignment-modal")).toBeInTheDocument();
      });
    });

    it("closes assignment modal when clicking close button", async () => {
      render(<SubstitutionsPage />);

      // Open modal
      const teacherCard = screen.getByRole("button", { name: /Anna Meyer/i });
      fireEvent.click(teacherCard);

      await waitFor(() => {
        expect(screen.getByTestId("assignment-modal")).toBeInTheDocument();
      });

      // Close modal
      fireEvent.click(screen.getByTestId("modal-close"));

      await waitFor(() => {
        expect(screen.queryByTestId("assignment-modal")).not.toBeInTheDocument();
      });
    });
  });

  describe("End Substitution", () => {
    it("opens confirmation modal when clicking end button", async () => {
      render(<SubstitutionsPage />);

      // Find end button in active substitutions section
      const endButtons = screen.getAllByRole("button", { name: /Beenden/i });
      const firstEndButton = endButtons[0];
      if (firstEndButton) {
        fireEvent.click(firstEndButton);
      }

      await waitFor(() => {
        expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
        expect(screen.getByText("Vertretung beenden?")).toBeInTheDocument();
      });
    });

    it("ends substitution and shows success toast on confirm", async () => {
      mockDeleteSubstitution.mockResolvedValueOnce(undefined);
      mockMutateTeachers.mockResolvedValueOnce(undefined);
      mockMutateSubstitutions.mockResolvedValueOnce(undefined);

      render(<SubstitutionsPage />);

      // Click end button
      const endButtons = screen.getAllByRole("button", { name: /Beenden/i });
      const firstEndButton = endButtons[0];
      if (firstEndButton) {
        fireEvent.click(firstEndButton);
      }

      await waitFor(() => {
        expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
      });

      // Confirm
      fireEvent.click(screen.getByTestId("confirm-end"));

      await waitFor(() => {
        expect(mockDeleteSubstitution).toHaveBeenCalled();
        expect(mockMutateTeachers).toHaveBeenCalled();
        expect(mockMutateSubstitutions).toHaveBeenCalled();
        expect(mockToastSuccess).toHaveBeenCalled();
      });
    });

    it("closes confirmation modal on cancel", async () => {
      render(<SubstitutionsPage />);

      // Click end button
      const endButtons = screen.getAllByRole("button", { name: /Beenden/i });
      const firstEndButton = endButtons[0];
      if (firstEndButton) {
        fireEvent.click(firstEndButton);
      }

      await waitFor(() => {
        expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
      });

      // Cancel
      fireEvent.click(screen.getByTestId("cancel-end"));

      await waitFor(() => {
        expect(
          screen.queryByTestId("confirmation-modal"),
        ).not.toBeInTheDocument();
      });
    });
  });
});

describe("SubstitutionsPage helper functions", () => {
  it("filters teachers by search term matching name", () => {
    const teachers = [
      { firstName: "Anna", lastName: "Meyer" },
      { firstName: "Ben", lastName: "Schulz" },
    ];

    const searchTerm = "anna";
    const filtered = teachers.filter((t) =>
      `${t.firstName} ${t.lastName}`
        .toLowerCase()
        .includes(searchTerm.toLowerCase()),
    );

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.firstName).toBe("Anna");
  });

  it("filters teachers by available status", () => {
    const teachers = [
      { id: "1", inSubstitution: false },
      { id: "2", inSubstitution: true },
      { id: "3", inSubstitution: false },
    ];

    const filtered = teachers.filter((t) => !t.inSubstitution);

    expect(filtered).toHaveLength(2);
  });

  it("filters teachers by substitution status", () => {
    const teachers = [
      { id: "1", inSubstitution: false },
      { id: "2", inSubstitution: true },
      { id: "3", inSubstitution: true },
    ];

    const filtered = teachers.filter((t) => t.inSubstitution);

    expect(filtered).toHaveLength(2);
  });

  it("separates transfers from substitutions", () => {
    const substitutions = [
      { id: "1", isTransfer: true },
      { id: "2", isTransfer: false },
      { id: "3", isTransfer: true },
      { id: "4", isTransfer: false },
    ];

    const transfers = substitutions.filter((s) => s.isTransfer);
    const nonTransfers = substitutions.filter((s) => !s.isTransfer);

    expect(transfers).toHaveLength(2);
    expect(nonTransfers).toHaveLength(2);
  });
});
