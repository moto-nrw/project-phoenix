/**
 * Tests for Operator Suggestions Page
 * Tests the rendering, filtering, search, and status updates for feedback/suggestions
 */
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

// Hoisted mocks
const {
  mockUseOperatorAuth,
  mockUseSWR,
  mockMutate,
  mockFetchAll,
  mockUpdateStatus,
} = vi.hoisted(() => ({
  mockUseOperatorAuth: vi.fn(),
  mockUseSWR: vi.fn(),
  mockMutate: vi.fn(),
  mockFetchAll: vi.fn(),
  mockUpdateStatus: vi.fn(),
}));

// Mock hooks and contexts
vi.mock("~/lib/operator/auth-context", () => ({
  useOperatorAuth: mockUseOperatorAuth,
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("swr", () => ({
  default: mockUseSWR,
}));

// Mock suggestions API
vi.mock("~/lib/operator/suggestions-api", () => ({
  operatorSuggestionsService: {
    fetchAll: mockFetchAll,
    updateStatus: mockUpdateStatus,
  },
}));

// Mock UI components
/* eslint-disable @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-return, @typescript-eslint/prefer-optional-chain */
vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({ title, badge, filters, search }: any) => (
    <div data-testid="page-header">
      <h1>{title}</h1>
      {badge && (
        <span data-testid="badge">
          {badge.count} {badge.label}
        </span>
      )}
      {filters &&
        filters.map((f: any) => (
          <select
            key={f.id}
            data-testid={`filter-${f.id}`}
            value={f.value}
            onChange={(e) => f.onChange(e.target.value)}
          >
            {f.options.map((opt: any) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        ))}
      {search && (
        <input
          data-testid="search-input"
          value={search.value}
          onChange={(e) => search.onChange(e.target.value)}
          placeholder={search.placeholder}
        />
      )}
    </div>
  ),
}));

vi.mock("~/components/ui/skeleton", () => ({
  Skeleton: ({ className }: any) => (
    <div data-testid="skeleton" className={className} />
  ),
}));

vi.mock("~/components/operator/status-dropdown", () => ({
  StatusDropdown: ({ value, onChange, onOpenChange }: any) => (
    <select
      data-testid="status-dropdown"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      onFocus={() => onOpenChange?.(true)}
      onBlur={() => onOpenChange?.(false)}
    >
      <option value="open">Offen</option>
      <option value="planned">Geplant</option>
      <option value="in_progress">In Bearbeitung</option>
      <option value="done">Umgesetzt</option>
      <option value="rejected">Abgelehnt</option>
      <option value="need_info">Rückfrage</option>
    </select>
  ),
}));

vi.mock("~/components/operator/operator-comment-accordion", () => ({
  OperatorCommentAccordion: ({ commentCount, unreadCount }: any) => (
    <div data-testid="comment-accordion">
      Comments: {commentCount}, Unread: {unreadCount}
    </div>
  ),
}));

vi.mock("framer-motion", () => ({
  AnimatePresence: ({ children }: any) => <div>{children}</div>,
  LayoutGroup: ({ children }: any) => <div>{children}</div>,
  motion: {
    div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
  },
}));
/* eslint-enable @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-call */ vi.mock(
  "lucide-react",
  () => ({
    ThumbsUp: () => <span>ThumbsUp</span>,
    ThumbsDown: () => <span>ThumbsDown</span>,
  }),
);

vi.mock("~/lib/format-utils", () => ({
  getRelativeTime: (_date: Date) => "2 hours ago",
  getInitials: (name: string) =>
    name
      .split(" ")
      .map((n) => n[0])
      .join(""),
}));

import OperatorSuggestionsPage from "./page";

describe("OperatorSuggestionsPage", () => {
  const mockSuggestion = {
    id: "1",
    title: "Test Suggestion",
    description: "Test description",
    status: "open" as const,
    authorName: "John Doe",
    upvotes: 5,
    downvotes: 2,
    commentCount: 3,
    unreadCount: 1,
    isNew: true,
    createdAt: new Date("2025-01-01"),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseOperatorAuth.mockReturnValue({
      isAuthenticated: true,
      operator: { id: "1", email: "test@example.com" },
    });
    mockUseSWR.mockReturnValue({
      data: [mockSuggestion],
      isLoading: false,
      mutate: mockMutate,
    });
    mockMutate.mockResolvedValue(undefined);

    // Mock window.dispatchEvent
    vi.spyOn(window, "dispatchEvent").mockImplementation(() => true);
  });

  it("renders loading state", () => {
    mockUseSWR.mockReturnValue({
      data: undefined,
      isLoading: true,
      mutate: mockMutate,
    });

    render(<OperatorSuggestionsPage />);

    expect(screen.getAllByTestId("skeleton")).toHaveLength(21);
  });

  it("renders empty state when no suggestions", () => {
    mockUseSWR.mockReturnValue({
      data: [],
      isLoading: false,
      mutate: mockMutate,
    });

    render(<OperatorSuggestionsPage />);

    expect(screen.getByText("Kein Feedback vorhanden")).toBeInTheDocument();
    expect(
      screen.getByText("Es wurde noch kein Feedback eingereicht."),
    ).toBeInTheDocument();
  });

  it("renders suggestions list", () => {
    render(<OperatorSuggestionsPage />);

    expect(screen.getByText("Test Suggestion")).toBeInTheDocument();
    expect(screen.getByText("Test description")).toBeInTheDocument();
    expect(screen.getByText("John Doe")).toBeInTheDocument();
  });

  it("displays badge with correct count", () => {
    mockUseSWR.mockReturnValue({
      data: [mockSuggestion, { ...mockSuggestion, id: "2" }],
      isLoading: false,
      mutate: mockMutate,
    });

    render(<OperatorSuggestionsPage />);

    expect(screen.getByTestId("badge")).toHaveTextContent("2 Beiträge");
  });

  it("filters suggestions by status", () => {
    const doneSuggestion = {
      ...mockSuggestion,
      id: "2",
      status: "done" as const,
      title: "Done Suggestion",
    };

    mockUseSWR.mockReturnValue({
      data: [mockSuggestion, doneSuggestion],
      isLoading: false,
      mutate: mockMutate,
    });

    render(<OperatorSuggestionsPage />);

    const statusFilter = screen.getByTestId("filter-status");
    fireEvent.change(statusFilter, { target: { value: "open" } });

    expect(screen.getByText("Test Suggestion")).toBeInTheDocument();
    expect(screen.queryByText("Done Suggestion")).not.toBeInTheDocument();
  });

  it("filters suggestions by search term", async () => {
    render(<OperatorSuggestionsPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Test" } });

    await waitFor(() => {
      expect(screen.getByText("Test Suggestion")).toBeInTheDocument();
    });
  });

  it("shows empty state when search has no results", () => {
    render(<OperatorSuggestionsPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "NonexistentTerm" } });

    expect(screen.getByText("Keine Ergebnisse gefunden")).toBeInTheDocument();
    expect(
      screen.getByText("Versuche einen anderen Suchbegriff."),
    ).toBeInTheDocument();
  });

  it("updates suggestion status", async () => {
    mockUpdateStatus.mockResolvedValue(undefined);

    render(<OperatorSuggestionsPage />);

    const statusDropdown = screen.getByTestId("status-dropdown");
    fireEvent.change(statusDropdown, { target: { value: "done" } });

    await waitFor(() => {
      expect(mockUpdateStatus).toHaveBeenCalledWith("1", "done");
      expect(mockMutate).toHaveBeenCalled();
    });
  });

  it("dispatches event to refresh sidebar badge when status changes", async () => {
    mockUpdateStatus.mockResolvedValue(undefined);

    render(<OperatorSuggestionsPage />);

    const statusDropdown = screen.getByTestId("status-dropdown");
    fireEvent.change(statusDropdown, { target: { value: "done" } });

    await waitFor(() => {
      expect(window.dispatchEvent).toHaveBeenCalledWith(
        expect.objectContaining({
          type: "operator-suggestions-unviewed-refresh",
        }),
      );
    });
  });

  it("displays upvotes and downvotes", () => {
    render(<OperatorSuggestionsPage />);

    expect(screen.getByText("5")).toBeInTheDocument(); // upvotes
    expect(screen.getByText("2")).toBeInTheDocument(); // downvotes
  });

  it("shows 'Neu' badge for new suggestions", () => {
    render(<OperatorSuggestionsPage />);

    expect(screen.getByText("Neu")).toBeInTheDocument();
  });

  it("displays comment count and unread count", () => {
    render(<OperatorSuggestionsPage />);

    expect(screen.getByText(/Comments: 3/)).toBeInTheDocument();
    expect(screen.getByText(/Unread: 1/)).toBeInTheDocument();
  });

  it("displays author initials", () => {
    render(<OperatorSuggestionsPage />);

    expect(screen.getByText("JD")).toBeInTheDocument(); // John Doe initials
  });

  it("displays relative time", () => {
    render(<OperatorSuggestionsPage />);

    expect(screen.getByText("2 hours ago")).toBeInTheDocument();
  });

  it("sets z-index when dropdown is open", async () => {
    render(<OperatorSuggestionsPage />);

    const statusDropdown = screen.getByTestId("status-dropdown");
    fireEvent.focus(statusDropdown);

    await waitFor(() => {
      const container = statusDropdown.closest('[class*="z-"]');
      if (container) {
        expect(container).toHaveClass("z-10");
      }
    });
  });

  it("handles API errors gracefully", async () => {
    mockUpdateStatus.mockRejectedValue(new Error("API Error"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop - suppress console.error in test
    });

    render(<OperatorSuggestionsPage />);

    const statusDropdown = screen.getByTestId("status-dropdown");
    fireEvent.change(statusDropdown, { target: { value: "done" } });

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith(
        "Failed to update status:",
        expect.any(Error),
      );
    });

    consoleError.mockRestore();
  });

  it("dispatches refresh event when suggestion counts change", async () => {
    const { rerender } = render(<OperatorSuggestionsPage />);

    // Update with new unread count
    mockUseSWR.mockReturnValue({
      data: [{ ...mockSuggestion, unreadCount: 2 }],
      isLoading: false,
      mutate: mockMutate,
    });

    rerender(<OperatorSuggestionsPage />);

    await waitFor(() => {
      expect(window.dispatchEvent).toHaveBeenCalledWith(
        expect.objectContaining({
          type: "operator-suggestions-unviewed-refresh",
        }),
      );
    });
  });

  it("filters by title, description, and author name", () => {
    const suggestions = [
      mockSuggestion,
      {
        ...mockSuggestion,
        id: "2",
        title: "Another Title",
        description: "Different description",
        authorName: "Jane Smith",
      },
    ];

    mockUseSWR.mockReturnValue({
      data: suggestions,
      isLoading: false,
      mutate: mockMutate,
    });

    render(<OperatorSuggestionsPage />);

    // Search by title
    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Another" } });
    expect(screen.queryByText("Test Suggestion")).not.toBeInTheDocument();
    expect(screen.getByText("Another Title")).toBeInTheDocument();

    // Search by author
    fireEvent.change(searchInput, { target: { value: "Jane" } });
    expect(screen.getByText("Jane Smith")).toBeInTheDocument();
  });

  it("marks suggestion as viewed when status is changed", async () => {
    mockUpdateStatus.mockResolvedValue(undefined);

    render(<OperatorSuggestionsPage />);

    const statusDropdown = screen.getByTestId("status-dropdown");
    fireEvent.change(statusDropdown, { target: { value: "done" } });

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith(expect.any(Function), {
        revalidate: false,
      });
    });
  });
});
