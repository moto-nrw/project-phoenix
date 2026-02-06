/**
 * Tests for Operator Suggestion Detail Page
 * Tests the rendering, comment functionality, and status updates for a single suggestion
 */
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

// Hoisted mocks
const {
  mockUseParams,
  mockUsePush,
  mockUseOperatorAuth,
  mockUseSWR,
  mockMutate,
  mockFetchById,
  mockUpdateStatus,
  mockAddComment,
  mockDeleteComment,
} = vi.hoisted(() => ({
  mockUseParams: vi.fn(),
  mockUsePush: vi.fn(),
  mockUseOperatorAuth: vi.fn(),
  mockUseSWR: vi.fn(),
  mockMutate: vi.fn(),
  mockFetchById: vi.fn(),
  mockUpdateStatus: vi.fn(),
  mockAddComment: vi.fn(),
  mockDeleteComment: vi.fn(),
}));

// Mock navigation
vi.mock("next/navigation", () => ({
  useParams: mockUseParams,
  useRouter: () => ({ push: mockUsePush }),
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
    fetchById: mockFetchById,
    updateStatus: mockUpdateStatus,
    addComment: mockAddComment,
    deleteComment: mockDeleteComment,
  },
}));

// Mock UI components
/* eslint-disable @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-return, @typescript-eslint/no-unsafe-call */
vi.mock("~/components/operator/status-dropdown", () => ({
  StatusDropdown: ({ value, onChange }: any) => (
    <select
      data-testid="status-dropdown"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    >
      <option value="pending">Ausstehend</option>
      <option value="approved">Genehmigt</option>
      <option value="rejected">Abgelehnt</option>
    </select>
  ),
}));

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({ isOpen, children, title, onClose, onConfirm }: any) =>
    isOpen ? (
      <div data-testid="confirmation-modal">
        <h2>{title}</h2>
        {children}
        <button onClick={onClose}>Cancel</button>
        <button onClick={onConfirm} data-testid="confirm-button">
          Confirm
        </button>
      </div>
    ) : null,
}));

vi.mock("~/components/ui/skeleton", () => ({
  Skeleton: ({ className }: any) => (
    <div data-testid="skeleton" className={className} />
  ),
}));
/* eslint-enable @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-assignment */ vi.mock(
  "lucide-react",
  () => ({
    ThumbsUp: () => <span>ThumbsUp</span>,
    ThumbsDown: () => <span>ThumbsDown</span>,
  }),
);

vi.mock("~/lib/format-utils", () => ({
  getRelativeTime: (_date: Date) => "2 hours ago",
}));

import OperatorSuggestionDetailPage from "./page";

describe("OperatorSuggestionDetailPage", () => {
  const mockComment = {
    id: "comment-1",
    content: "Test comment",
    authorName: "Admin User",
    authorType: "operator" as const,
    createdAt: new Date("2025-01-02"),
  };

  const mockSuggestion = {
    id: "1",
    title: "Test Suggestion",
    description: "Test description\nWith multiple lines",
    status: "pending" as const,
    authorName: "John Doe",
    upvotes: 5,
    downvotes: 2,
    createdAt: new Date("2025-01-01"),
    operatorComments: [mockComment],
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseParams.mockReturnValue({ id: "1" });
    mockUseOperatorAuth.mockReturnValue({
      isAuthenticated: true,
      operator: { id: "1", email: "test@example.com" },
    });
    mockUseSWR.mockReturnValue({
      data: mockSuggestion,
      isLoading: false,
      mutate: mockMutate,
    });
    mockMutate.mockResolvedValue(undefined);
  });

  it("renders loading state", () => {
    mockUseSWR.mockReturnValue({
      data: undefined,
      isLoading: true,
      mutate: mockMutate,
    });

    render(<OperatorSuggestionDetailPage />);

    expect(screen.getAllByTestId("skeleton").length).toBeGreaterThan(0);
  });

  it("renders not found state when suggestion is null", () => {
    mockUseSWR.mockReturnValue({
      data: null,
      isLoading: false,
      mutate: mockMutate,
    });

    render(<OperatorSuggestionDetailPage />);

    expect(screen.getByText("Feedback nicht gefunden")).toBeInTheDocument();
    expect(screen.getByText("Zurück zur Übersicht")).toBeInTheDocument();
  });

  it("navigates back to suggestions list from not found state", () => {
    mockUseSWR.mockReturnValue({
      data: null,
      isLoading: false,
      mutate: mockMutate,
    });

    render(<OperatorSuggestionDetailPage />);

    const backButton = screen.getByText("Zurück zur Übersicht");
    fireEvent.click(backButton);

    expect(mockUsePush).toHaveBeenCalledWith("/operator/suggestions");
  });

  it("renders suggestion details", () => {
    render(<OperatorSuggestionDetailPage />);

    expect(screen.getByText("Test Suggestion")).toBeInTheDocument();
    expect(
      screen.getByText(
        (_content, element) =>
          element?.textContent === "Test description\nWith multiple lines",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("John Doe")).toBeInTheDocument();
    expect(screen.getAllByText("2 hours ago").length).toBeGreaterThanOrEqual(1);
  });

  it("displays upvotes and downvotes", () => {
    render(<OperatorSuggestionDetailPage />);

    expect(screen.getByText("5")).toBeInTheDocument(); // upvotes
    expect(screen.getByText("2")).toBeInTheDocument(); // downvotes
  });

  it("renders back button", () => {
    render(<OperatorSuggestionDetailPage />);

    const backButton = screen.getByText("Zurück");
    expect(backButton).toBeInTheDocument();
  });

  it("navigates back to suggestions list", () => {
    render(<OperatorSuggestionDetailPage />);

    const backButton = screen.getByText("Zurück");
    fireEvent.click(backButton);

    expect(mockUsePush).toHaveBeenCalledWith("/operator/suggestions");
  });

  it("updates suggestion status", async () => {
    mockUpdateStatus.mockResolvedValue(undefined);

    render(<OperatorSuggestionDetailPage />);

    const statusDropdown = screen.getByTestId("status-dropdown");
    fireEvent.change(statusDropdown, { target: { value: "approved" } });

    await waitFor(() => {
      expect(mockUpdateStatus).toHaveBeenCalledWith("1", "approved");
      expect(mockMutate).toHaveBeenCalled();
    });
  });

  it("renders comments section", () => {
    render(<OperatorSuggestionDetailPage />);

    expect(screen.getByText("Kommentare (1)")).toBeInTheDocument();
    expect(screen.getByText("Test comment")).toBeInTheDocument();
    expect(screen.getByText("Admin User")).toBeInTheDocument();
  });

  it("displays operator badge for operator comments", () => {
    render(<OperatorSuggestionDetailPage />);

    expect(screen.getByText("moto Team")).toBeInTheDocument();
  });

  it("displays user badge for user comments", () => {
    const userComment = {
      ...mockComment,
      id: "comment-2",
      authorType: "user" as const,
    };

    mockUseSWR.mockReturnValue({
      data: {
        ...mockSuggestion,
        operatorComments: [userComment],
      },
      isLoading: false,
      mutate: mockMutate,
    });

    render(<OperatorSuggestionDetailPage />);

    expect(screen.getByText("OGS-Benutzer")).toBeInTheDocument();
  });

  it("adds new comment", async () => {
    mockAddComment.mockResolvedValue(undefined);

    render(<OperatorSuggestionDetailPage />);

    const textarea = screen.getByPlaceholderText("Kommentar schreiben...");
    fireEvent.change(textarea, { target: { value: "New comment" } });

    const submitButton = screen.getByText("Senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockAddComment).toHaveBeenCalledWith("1", "New comment", false);
      expect(mockMutate).toHaveBeenCalled();
    });
  });

  it("clears textarea after submitting comment", async () => {
    mockAddComment.mockResolvedValue(undefined);

    render(<OperatorSuggestionDetailPage />);

    // eslint-disable-next-line @typescript-eslint/no-unnecessary-type-assertion
    const textarea = screen.getByPlaceholderText(
      "Kommentar schreiben...",
    ) as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: "New comment" } });

    const submitButton = screen.getByText("Senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(textarea.value).toBe("");
    });
  });

  it("disables submit button when comment is empty", () => {
    render(<OperatorSuggestionDetailPage />);

    const submitButton = screen.getByText("Senden");
    expect(submitButton).toBeDisabled();
  });

  it("opens delete comment confirmation modal", async () => {
    render(<OperatorSuggestionDetailPage />);

    const deleteButton = screen.getByLabelText("Kommentar löschen");
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
      expect(screen.getByText("Kommentar löschen?")).toBeInTheDocument();
    });
  });

  it("deletes comment after confirmation", async () => {
    mockDeleteComment.mockResolvedValue(undefined);

    render(<OperatorSuggestionDetailPage />);

    // Open delete modal
    const deleteButton = screen.getByLabelText("Kommentar löschen");
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    // Confirm deletion
    const confirmButton = screen.getByTestId("confirm-button");
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(mockDeleteComment).toHaveBeenCalledWith("1", "comment-1");
      expect(mockMutate).toHaveBeenCalled();
    });
  });

  it("closes delete modal on cancel", async () => {
    render(<OperatorSuggestionDetailPage />);

    // Open delete modal
    const deleteButton = screen.getByLabelText("Kommentar löschen");
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    // Cancel
    const cancelButton = screen.getByText("Cancel");
    fireEvent.click(cancelButton);

    await waitFor(() => {
      expect(
        screen.queryByTestId("confirmation-modal"),
      ).not.toBeInTheDocument();
    });
  });

  it("handles comment submission errors gracefully", async () => {
    mockAddComment.mockRejectedValue(new Error("API Error"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop - suppress console.error in test
    });

    render(<OperatorSuggestionDetailPage />);

    const textarea = screen.getByPlaceholderText("Kommentar schreiben...");
    fireEvent.change(textarea, { target: { value: "New comment" } });

    const submitButton = screen.getByText("Senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith(
        "Failed to add comment:",
        expect.any(Error),
      );
    });

    consoleError.mockRestore();
  });

  it("handles status update errors gracefully", async () => {
    mockUpdateStatus.mockRejectedValue(new Error("API Error"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop - suppress console.error in test
    });

    render(<OperatorSuggestionDetailPage />);

    const statusDropdown = screen.getByTestId("status-dropdown");
    fireEvent.change(statusDropdown, { target: { value: "approved" } });

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith(
        "Failed to update status:",
        expect.any(Error),
      );
    });

    consoleError.mockRestore();
  });

  it("handles comment deletion errors gracefully", async () => {
    mockDeleteComment.mockRejectedValue(new Error("API Error"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop - suppress console.error in test
    });

    render(<OperatorSuggestionDetailPage />);

    // Open delete modal
    const deleteButton = screen.getByLabelText("Kommentar löschen");
    fireEvent.click(deleteButton);

    await waitFor(() => {
      const confirmButton = screen.getByTestId("confirm-button");
      fireEvent.click(confirmButton);
    });

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith(
        "Failed to delete comment:",
        expect.any(Error),
      );
    });

    consoleError.mockRestore();
  });

  it("renders multiple comments", () => {
    const comments = [
      mockComment,
      {
        id: "comment-2",
        content: "Second comment",
        authorName: "Another User",
        authorType: "user" as const,
        createdAt: new Date("2025-01-03"),
      },
    ];

    mockUseSWR.mockReturnValue({
      data: {
        ...mockSuggestion,
        operatorComments: comments,
      },
      isLoading: false,
      mutate: mockMutate,
    });

    render(<OperatorSuggestionDetailPage />);

    expect(screen.getByText("Kommentare (2)")).toBeInTheDocument();
    expect(screen.getByText("Test comment")).toBeInTheDocument();
    expect(screen.getByText("Second comment")).toBeInTheDocument();
  });

  it("shows loading text during submission", async () => {
    mockAddComment.mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 100)),
    );

    render(<OperatorSuggestionDetailPage />);

    const textarea = screen.getByPlaceholderText("Kommentar schreiben...");
    fireEvent.change(textarea, { target: { value: "New comment" } });

    const submitButton = screen.getByText("Senden");
    fireEvent.click(submitButton);

    expect(screen.getByText("Wird gesendet...")).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.queryByText("Wird gesendet...")).not.toBeInTheDocument();
    });
  });

  it("preserves whitespace in description", () => {
    render(<OperatorSuggestionDetailPage />);

    const description = screen.getByText(
      (_content, element) =>
        element?.tagName === "P" &&
        element.textContent === "Test description\nWith multiple lines",
    );
    expect(description).toHaveClass("whitespace-pre-wrap");
  });

  it("preserves whitespace in comments", () => {
    render(<OperatorSuggestionDetailPage />);

    const comment = screen.getByText("Test comment");
    expect(comment).toHaveClass("whitespace-pre-wrap");
  });
});
