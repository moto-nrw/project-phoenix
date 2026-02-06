import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { BaseCommentAccordion } from "./base-comment-accordion";
import type { BaseComment } from "./base-comment-accordion";

// Mock UI components
vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: vi.fn(
    ({
      isOpen,
      onConfirm,
      children,
    }: {
      isOpen: boolean;
      onConfirm: () => void;
      children: React.ReactNode;
    }) =>
      isOpen ? (
        <div data-testid="confirmation-modal">
          {children}
          <button onClick={onConfirm} data-testid="confirm-delete">
            Confirm
          </button>
        </div>
      ) : null,
  ),
}));

// Mock format utils
vi.mock("~/lib/format-utils", () => ({
  getRelativeTime: vi.fn((_date: string) => "vor 2 Stunden"),
  getInitial: vi.fn((name: string) => name.charAt(0).toUpperCase()),
}));

describe("BaseCommentAccordion", () => {
  const mockComments: BaseComment[] = [
    {
      id: "1",
      content: "Test comment 1",
      authorName: "Alice",
      authorType: "operator",
      createdAt: "2024-01-01T10:00:00Z",
    },
    {
      id: "2",
      content: "Test comment 2",
      authorName: "Bob",
      authorType: "user",
      createdAt: "2024-01-01T11:00:00Z",
    },
  ];

  const mockLoadComments = vi.fn();
  const mockCreateComment = vi.fn();
  const mockDeleteComment = vi.fn();
  const mockOnOpen = vi.fn();
  const mockOnAfterCreate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockLoadComments.mockResolvedValue(mockComments);
    mockCreateComment.mockResolvedValue(undefined);
    mockDeleteComment.mockResolvedValue(undefined);
  });

  it("renders accordion header with comment count", () => {
    render(
      <BaseCommentAccordion
        postId="42"
        commentCount={5}
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
      />,
    );

    expect(screen.getByText("Kommentare (5)")).toBeInTheDocument();
  });

  it("shows unread badge when unreadCount > 0", () => {
    render(
      <BaseCommentAccordion
        postId="42"
        unreadCount={3}
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
      />,
    );

    expect(screen.getByText("3 neu")).toBeInTheDocument();
  });

  it("does not show unread badge when unreadCount is 0", () => {
    render(
      <BaseCommentAccordion
        postId="42"
        unreadCount={0}
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
      />,
    );

    expect(screen.queryByText("neu")).not.toBeInTheDocument();
  });

  it("loads comments on accordion open", async () => {
    render(
      <BaseCommentAccordion
        postId="42"
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
      />,
    );

    const button = screen.getByRole("button", { name: /Kommentare/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(mockLoadComments).toHaveBeenCalledWith("42");
    });
  });

  it("displays comments after loading", async () => {
    render(
      <BaseCommentAccordion
        postId="42"
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
      />,
    );

    const button = screen.getByRole("button", { name: /Kommentare/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByText("Test comment 1")).toBeInTheDocument();
      expect(screen.getByText("Test comment 2")).toBeInTheDocument();
    });
  });

  it("shows loading state while fetching", async () => {
    mockLoadComments.mockImplementation(
      () => new Promise((resolve) => setTimeout(() => resolve([]), 100)),
    );

    render(
      <BaseCommentAccordion
        postId="42"
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
      />,
    );

    const button = screen.getByRole("button", { name: /Kommentare/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByText("Laden...")).toBeInTheDocument();
    });
  });

  it("shows error message on load failure", async () => {
    mockLoadComments.mockRejectedValue(new Error("Network error"));

    render(
      <BaseCommentAccordion
        postId="42"
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
      />,
    );

    const button = screen.getByRole("button", { name: /Kommentare/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(
        screen.getByText("Kommentare konnten nicht geladen werden."),
      ).toBeInTheDocument();
    });
  });

  it("submits new comment", async () => {
    render(
      <BaseCommentAccordion
        postId="42"
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
      />,
    );

    const button = screen.getByRole("button", { name: /Kommentare/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(
        screen.getByPlaceholderText("Kommentar schreiben..."),
      ).toBeInTheDocument();
    });

    const textarea = screen.getByPlaceholderText("Kommentar schreiben...");
    const submitButton = screen.getByLabelText("Senden");

    fireEvent.change(textarea, { target: { value: "New comment" } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockCreateComment).toHaveBeenCalledWith("42", "New comment");
    });
  });

  it("clears textarea after successful submit", async () => {
    render(
      <BaseCommentAccordion
        postId="42"
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
      />,
    );

    const button = screen.getByRole("button", { name: /Kommentare/i });
    fireEvent.click(button);

    await waitFor(() => {
      const textarea = screen.getByPlaceholderText("Kommentar schreiben...");
      fireEvent.change(textarea, { target: { value: "New comment" } });
    });

    const submitButton = screen.getByLabelText("Senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      const textarea = screen.getByPlaceholderText("Kommentar schreiben...");
      expect(textarea).toHaveValue("");
    });
  });

  it("does not submit empty comment", async () => {
    render(
      <BaseCommentAccordion
        postId="42"
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
      />,
    );

    const button = screen.getByRole("button", { name: /Kommentare/i });
    fireEvent.click(button);

    await waitFor(() => {
      const submitButton = screen.getByLabelText("Senden");
      expect(submitButton).toBeDisabled();
    });
  });

  it("shows delete button for comments", async () => {
    render(
      <BaseCommentAccordion
        postId="42"
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
      />,
    );

    const button = screen.getByRole("button", { name: /Kommentare/i });
    fireEvent.click(button);

    await waitFor(() => {
      const deleteButtons = screen.getAllByLabelText("Kommentar löschen");
      expect(deleteButtons).toHaveLength(2);
    });
  });

  it("opens delete confirmation modal", async () => {
    render(
      <BaseCommentAccordion
        postId="42"
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
      />,
    );

    const button = screen.getByRole("button", { name: /Kommentare/i });
    fireEvent.click(button);

    await waitFor(() => {
      const deleteButtons = screen.getAllByLabelText("Kommentar löschen");
      fireEvent.click(deleteButtons[0]!);
    });

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });
  });

  it("deletes comment after confirmation", async () => {
    render(
      <BaseCommentAccordion
        postId="42"
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
      />,
    );

    const button = screen.getByRole("button", { name: /Kommentare/i });
    fireEvent.click(button);

    await waitFor(() => {
      const deleteButtons = screen.getAllByLabelText("Kommentar löschen");
      fireEvent.click(deleteButtons[0]!);
    });

    await waitFor(() => {
      const confirmButton = screen.getByTestId("confirm-delete");
      fireEvent.click(confirmButton);
    });

    await waitFor(() => {
      expect(mockDeleteComment).toHaveBeenCalledWith("42", "1");
    });
  });

  it("calls onOpen callback when opening", async () => {
    render(
      <BaseCommentAccordion
        postId="42"
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
        onOpen={mockOnOpen}
      />,
    );

    const button = screen.getByRole("button", { name: /Kommentare/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(mockOnOpen).toHaveBeenCalledWith("42");
    });
  });

  it("calls onAfterCreate after creating comment", async () => {
    mockOnAfterCreate.mockResolvedValue(undefined);

    render(
      <BaseCommentAccordion
        postId="42"
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
        onAfterCreate={mockOnAfterCreate}
      />,
    );

    const button = screen.getByRole("button", { name: /Kommentare/i });
    fireEvent.click(button);

    await waitFor(() => {
      const textarea = screen.getByPlaceholderText("Kommentar schreiben...");
      fireEvent.change(textarea, { target: { value: "New comment" } });
    });

    const submitButton = screen.getByLabelText("Senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockOnAfterCreate).toHaveBeenCalled();
    });
  });

  it("uses canDeleteComment to filter delete buttons", async () => {
    const canDeleteComment = vi.fn(
      (comment: BaseComment) => comment.authorType === "operator",
    );

    render(
      <BaseCommentAccordion
        postId="42"
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
        canDeleteComment={canDeleteComment}
      />,
    );

    const button = screen.getByRole("button", { name: /Kommentare/i });
    fireEvent.click(button);

    await waitFor(() => {
      const deleteButtons = screen.getAllByLabelText("Kommentar löschen");
      expect(deleteButtons).toHaveLength(1); // Only operator comment
    });
  });

  it("shows empty state when no comments", async () => {
    mockLoadComments.mockResolvedValue([]);

    render(
      <BaseCommentAccordion
        postId="42"
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
      />,
    );

    const button = screen.getByRole("button", { name: /Kommentare/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByText("Noch keine Kommentare.")).toBeInTheDocument();
    });
  });

  it("submits comment on Enter key (without Shift)", async () => {
    render(
      <BaseCommentAccordion
        postId="42"
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
      />,
    );

    const button = screen.getByRole("button", { name: /Kommentare/i });
    fireEvent.click(button);

    await waitFor(() => {
      const textarea = screen.getByPlaceholderText("Kommentar schreiben...");
      fireEvent.change(textarea, { target: { value: "New comment" } });
      fireEvent.keyDown(textarea, { key: "Enter", shiftKey: false });
    });

    await waitFor(() => {
      expect(mockCreateComment).toHaveBeenCalledWith("42", "New comment");
    });
  });

  it("does not submit on Shift+Enter", async () => {
    render(
      <BaseCommentAccordion
        postId="42"
        loadComments={mockLoadComments}
        createComment={mockCreateComment}
        deleteComment={mockDeleteComment}
      />,
    );

    const button = screen.getByRole("button", { name: /Kommentare/i });
    fireEvent.click(button);

    await waitFor(() => {
      const textarea = screen.getByPlaceholderText("Kommentar schreiben...");
      fireEvent.change(textarea, { target: { value: "New comment" } });
      fireEvent.keyDown(textarea, { key: "Enter", shiftKey: true });
    });

    await waitFor(() => {
      expect(mockCreateComment).not.toHaveBeenCalled();
    });
  });
});
