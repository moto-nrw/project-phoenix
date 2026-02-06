import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import SuggestionsPage from "./page";
import type { Suggestion } from "~/lib/suggestions-helpers";

// ============================================================================
// Mocks
// ============================================================================

const mockPush = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
    replace: vi.fn(),
  }),
}));

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: {
      user: {
        id: "10",
        name: "Test User",
        email: "user@test.com",
        token: "test-token",
        isAdmin: false,
        firstName: "Test",
      },
      expires: "2099-12-31",
    },
    status: "authenticated",
  })),
}));

const mockMutate = vi.fn();

const mockUseSWRAuth = vi.hoisted(() => vi.fn());

vi.mock("~/lib/swr", () => ({
  useSWRAuth: mockUseSWRAuth,
}));

vi.mock("~/lib/suggestions-api", () => ({
  fetchSuggestions: vi.fn(),
  deleteSuggestion: vi.fn(),
  createSuggestion: vi.fn(),
  updateSuggestion: vi.fn(),
  voteSuggestion: vi.fn(),
  removeVote: vi.fn(),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  }),
}));

vi.mock("~/components/ui/skeleton", () => ({
  Skeleton: ({
    className,
    ...props
  }: { className?: string } & Record<string, unknown>) => (
    <div
      data-testid="skeleton"
      className={className}
      {...(props as React.HTMLAttributes<HTMLDivElement>)}
    />
  ),
}));

vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({
    title,
    search,
    actionButton,
  }: {
    title: string;
    badge?: { count: number; label: string };
    filters?: unknown[];
    search?: {
      value: string;
      onChange: (v: string) => void;
      placeholder: string;
    };
    actionButton?: React.ReactNode;
    mobileActionButton?: React.ReactNode;
  }) => (
    <div data-testid="page-header">
      <h1>{title}</h1>
      {search && (
        <input
          data-testid="search-input"
          value={search.value}
          onChange={(e) => search.onChange(e.target.value)}
          placeholder={search.placeholder}
        />
      )}
      {actionButton}
    </div>
  ),
}));

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    children,
    title,
    footer,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <h2>{title}</h2>
        {children}
        {footer && <div data-testid="modal-footer">{footer}</div>}
      </div>
    ) : null,
  ConfirmationModal: ({
    isOpen,
    onConfirm,
    onClose,
    children,
    title,
  }: {
    isOpen: boolean;
    onConfirm: () => void;
    onClose: () => void;
    children: React.ReactNode;
    title: string;
    confirmText?: string;
    confirmButtonClass?: string;
    isConfirmLoading?: boolean;
  }) =>
    isOpen ? (
      <div data-testid="confirmation-modal">
        <h2>{title}</h2>
        {children}
        <button onClick={onConfirm} data-testid="confirm-delete">
          Löschen
        </button>
        <button onClick={onClose} data-testid="cancel-delete">
          Abbrechen
        </button>
      </div>
    ) : null,
}));

vi.mock("framer-motion", () => ({
  AnimatePresence: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
  LayoutGroup: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  motion: {
    div: ({
      children,
      ...props
    }: { children: React.ReactNode } & Record<string, unknown>) => (
      <div {...(props as React.HTMLAttributes<HTMLDivElement>)}>{children}</div>
    ),
  },
}));

// ============================================================================
// Test data
// ============================================================================

const sampleSuggestion: Suggestion = {
  id: "1",
  title: "PDF-Export Feature",
  description: "Wir brauchen einen PDF-Export.",
  authorId: "10",
  authorName: "Test User",
  status: "open",
  score: 5,
  upvotes: 7,
  downvotes: 2,
  commentCount: 0,
  userVote: null,
  createdAt: new Date(Date.now() - 3600000).toISOString(),
  updatedAt: new Date().toISOString(),
};

const anotherSuggestion: Suggestion = {
  id: "2",
  title: "Darkmode Support",
  description: "Bitte Darkmode hinzufügen.",
  authorId: "20",
  authorName: "Other User",
  status: "planned",
  score: 3,
  upvotes: 4,
  downvotes: 1,
  commentCount: 0,
  userVote: "up",
  createdAt: new Date(Date.now() - 7200000).toISOString(),
  updatedAt: new Date().toISOString(),
};

// ============================================================================
// Tests
// ============================================================================

describe("SuggestionsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default: return suggestions
    mockUseSWRAuth.mockReturnValue({
      data: [sampleSuggestion, anotherSuggestion],
      isLoading: false,
      mutate: mockMutate,
      error: undefined,
      isValidating: false,
    });
  });

  it("renders page header with title", async () => {
    render(<SuggestionsPage />);

    await waitFor(() => {
      expect(screen.getByText("Feedback")).toBeInTheDocument();
    });
  });

  it("renders suggestion cards", async () => {
    render(<SuggestionsPage />);

    await waitFor(() => {
      expect(screen.getByText("PDF-Export Feature")).toBeInTheDocument();
      expect(screen.getByText("Darkmode Support")).toBeInTheDocument();
    });
  });

  it("renders new post button when suggestions exist", async () => {
    render(<SuggestionsPage />);

    await waitFor(() => {
      expect(screen.getByText("Neuer Beitrag")).toBeInTheDocument();
    });
  });

  it("shows empty state when no suggestions", async () => {
    mockUseSWRAuth.mockReturnValue({
      data: [],
      isLoading: false,
      mutate: mockMutate,
      error: undefined,
      isValidating: false,
    });

    render(<SuggestionsPage />);

    await waitFor(() => {
      expect(screen.getByText("Noch keine Beiträge")).toBeInTheDocument();
    });
  });

  it("shows skeleton loading state", async () => {
    mockUseSWRAuth.mockReturnValue({
      data: undefined,
      isLoading: true,
      mutate: mockMutate,
      error: undefined,
      isValidating: false,
    });

    render(<SuggestionsPage />);

    await waitFor(() => {
      const skeletons = screen.getAllByTestId("skeleton");
      expect(skeletons.length).toBeGreaterThan(0);
    });
  });

  it("shows owner actions for own posts only", async () => {
    render(<SuggestionsPage />);

    await waitFor(() => {
      // accountId is "10" and only sampleSuggestion (authorId "10") is owned
      const actionButtons = screen.getAllByLabelText("Aktionen");
      expect(actionButtons).toHaveLength(1);
    });
  });

  it("opens form when Neuer Beitrag is clicked", async () => {
    render(<SuggestionsPage />);

    await waitFor(() => {
      expect(screen.getByText("Neuer Beitrag")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Neuer Beitrag"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
  });

  it("shows search input", async () => {
    render(<SuggestionsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("search-input")).toBeInTheDocument();
    });
  });

  it("shows search empty state when no results match", async () => {
    render(<SuggestionsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("search-input")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "nonexistentterm" },
    });

    await waitFor(() => {
      expect(screen.getByText("Keine Ergebnisse gefunden")).toBeInTheDocument();
    });
  });

  it("opens empty state create button", async () => {
    mockUseSWRAuth.mockReturnValue({
      data: [],
      isLoading: false,
      mutate: mockMutate,
      error: undefined,
      isValidating: false,
    });

    render(<SuggestionsPage />);

    await waitFor(() => {
      expect(screen.getByText("Neuer Beitrag")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Neuer Beitrag"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
  });
});
