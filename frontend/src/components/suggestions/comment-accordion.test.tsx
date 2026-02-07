import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { CommentAccordion } from "./comment-accordion";
import type { BaseComment } from "~/components/shared/base-comment-accordion";

// Mock the suggestions API
const mockFetchComments = vi.fn();
const mockCreateComment = vi.fn();
const mockDeleteComment = vi.fn();
const mockMarkCommentsRead = vi.fn();

vi.mock("~/lib/suggestions-api", () => ({
  fetchComments: (...args: unknown[]) => mockFetchComments(...args) as unknown,
  createComment: (...args: unknown[]) => mockCreateComment(...args) as unknown,
  deleteComment: (...args: unknown[]) => mockDeleteComment(...args) as unknown,
  markCommentsRead: (...args: unknown[]) =>
    mockMarkCommentsRead(...args) as unknown,
}));

// Mock BaseCommentAccordion
vi.mock("~/components/shared/base-comment-accordion", async (importActual) => {
  const actual = await importActual<Record<string, unknown>>();
  return {
    ...actual,
    BaseCommentAccordion: ({
      postId,
      commentCount,
      unreadCount,
      loadComments,
      createComment,
      deleteComment,
      onOpen,
      onAfterCreate,
      canDeleteComment,
    }: {
      postId: string;
      commentCount?: number;
      unreadCount?: number;
      loadComments: (postId: string) => Promise<BaseComment[]>;
      createComment: (postId: string, content: string) => Promise<void>;
      deleteComment: (postId: string, commentId: string) => Promise<void>;
      onOpen?: (postId: string) => void;
      onAfterCreate?: (
        postId: string,
        reloadComments: () => Promise<void>,
      ) => Promise<void>;
      canDeleteComment?: (comment: BaseComment) => boolean;
    }) => (
      <div data-testid="base-comment-accordion">
        <button
          data-testid="accordion-toggle"
          onClick={() => {
            onOpen?.(postId);
            void loadComments(postId);
          }}
        >
          Toggle Accordion (Comments: {commentCount ?? 0}, Unread:{" "}
          {unreadCount ?? 0})
        </button>
        <button
          data-testid="create-comment"
          onClick={async () => {
            await createComment(postId, "New comment");
            await onAfterCreate?.(postId, async () => {
              await loadComments(postId);
            });
          }}
        >
          Create Comment
        </button>
        <button
          data-testid="delete-comment"
          onClick={() => void deleteComment(postId, "comment-1")}
        >
          Delete Comment
        </button>
        <div data-testid="can-delete-user">
          {String(
            canDeleteComment?.({
              id: "1",
              content: "Test",
              authorId: "42",
              authorName: "User",
              authorType: "user",
              createdAt: "2024-01-01",
            }),
          )}
        </div>
        <div data-testid="can-delete-operator">
          {String(
            canDeleteComment?.({
              id: "2",
              content: "Test",
              authorId: "42",
              authorName: "Operator",
              authorType: "operator",
              createdAt: "2024-01-01",
            }),
          )}
        </div>
      </div>
    ),
  };
});

describe("CommentAccordion", () => {
  const mockComments: BaseComment[] = [
    {
      id: "1",
      content: "Test comment",
      authorId: "42",
      authorName: "Alice",
      authorType: "user",
      createdAt: "2024-01-01T10:00:00Z",
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchComments.mockResolvedValue(mockComments);
    mockCreateComment.mockResolvedValue(undefined);
    mockDeleteComment.mockResolvedValue(undefined);
    mockMarkCommentsRead.mockResolvedValue(undefined);

    // Reset event listeners
    window.dispatchEvent = vi.fn();
  });

  it("renders BaseCommentAccordion with correct props", () => {
    render(
      <CommentAccordion
        postId="123"
        currentAccountId="42"
        commentCount={5}
        unreadCount={2}
      />,
    );

    expect(screen.getByTestId("base-comment-accordion")).toBeInTheDocument();
    expect(screen.getByText(/Comments: 5, Unread: 2/)).toBeInTheDocument();
  });

  it("passes fetch/create/delete functions to BaseCommentAccordion", async () => {
    render(<CommentAccordion postId="123" currentAccountId="42" />);

    // Test loadComments
    const toggleButton = screen.getByTestId("accordion-toggle");
    fireEvent.click(toggleButton);

    await waitFor(() => {
      expect(mockFetchComments).toHaveBeenCalledWith("123");
    });

    // Test createComment
    const createButton = screen.getByTestId("create-comment");
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(mockCreateComment).toHaveBeenCalledWith("123", "New comment");
    });

    // Test deleteComment
    const deleteButton = screen.getByTestId("delete-comment");
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(mockDeleteComment).toHaveBeenCalledWith("123", "comment-1");
    });
  });

  it("marks comments as read when accordion is opened with unread count", async () => {
    render(
      <CommentAccordion postId="123" currentAccountId="42" unreadCount={3} />,
    );

    const toggleButton = screen.getByTestId("accordion-toggle");
    fireEvent.click(toggleButton);

    await waitFor(() => {
      expect(mockMarkCommentsRead).toHaveBeenCalledWith("123");
    });
  });

  it("does not mark comments as read when unread count is 0", async () => {
    render(
      <CommentAccordion postId="123" currentAccountId="42" unreadCount={0} />,
    );

    const toggleButton = screen.getByTestId("accordion-toggle");
    fireEvent.click(toggleButton);

    await waitFor(() => {
      expect(mockFetchComments).toHaveBeenCalled();
    });

    expect(mockMarkCommentsRead).not.toHaveBeenCalled();
  });

  it("does not mark comments as read when no unread count is provided", async () => {
    render(<CommentAccordion postId="123" currentAccountId="42" />);

    const toggleButton = screen.getByTestId("accordion-toggle");
    fireEvent.click(toggleButton);

    await waitFor(() => {
      expect(mockFetchComments).toHaveBeenCalled();
    });

    expect(mockMarkCommentsRead).not.toHaveBeenCalled();
  });

  it("only marks comments as read once", async () => {
    render(
      <CommentAccordion postId="123" currentAccountId="42" unreadCount={2} />,
    );

    const toggleButton = screen.getByTestId("accordion-toggle");

    // First open - should mark as read
    fireEvent.click(toggleButton);
    await waitFor(() => {
      expect(mockMarkCommentsRead).toHaveBeenCalledTimes(1);
    });

    // Second open - should not mark as read again
    fireEvent.click(toggleButton);
    fireEvent.click(toggleButton);

    await waitFor(() => {
      expect(mockFetchComments).toHaveBeenCalled();
    });

    expect(mockMarkCommentsRead).toHaveBeenCalledTimes(1);
  });

  it("dispatches suggestions-unread-refresh event after marking as read", async () => {
    const dispatchEventSpy = vi.spyOn(window, "dispatchEvent");

    render(
      <CommentAccordion postId="123" currentAccountId="42" unreadCount={1} />,
    );

    const toggleButton = screen.getByTestId("accordion-toggle");
    fireEvent.click(toggleButton);

    await waitFor(() => {
      expect(mockMarkCommentsRead).toHaveBeenCalled();
    });

    await waitFor(() => {
      expect(dispatchEventSpy).toHaveBeenCalledWith(
        expect.objectContaining({
          type: "suggestions-unread-refresh",
        }),
      );
    });

    dispatchEventSpy.mockRestore();
  });

  it("marks comments as read and dispatches event after creating a comment", async () => {
    const dispatchEventSpy = vi.spyOn(window, "dispatchEvent");

    render(<CommentAccordion postId="123" currentAccountId="42" />);

    const createButton = screen.getByTestId("create-comment");
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(mockCreateComment).toHaveBeenCalled();
      expect(mockMarkCommentsRead).toHaveBeenCalledWith("123");
    });

    await waitFor(() => {
      expect(dispatchEventSpy).toHaveBeenCalledWith(
        expect.objectContaining({
          type: "suggestions-unread-refresh",
        }),
      );
    });

    dispatchEventSpy.mockRestore();
  });

  it("reloads comments after creating a comment", async () => {
    render(<CommentAccordion postId="123" currentAccountId="42" />);

    const createButton = screen.getByTestId("create-comment");
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(mockCreateComment).toHaveBeenCalled();
    });

    // Should call loadComments twice: once for onAfterCreate reload
    await waitFor(() => {
      expect(mockFetchComments).toHaveBeenCalled();
    });
  });

  it("allows deletion of comments by user", () => {
    render(<CommentAccordion postId="123" currentAccountId="42" />);

    expect(screen.getByTestId("can-delete-user")).toHaveTextContent("true");
  });

  it("does not allow deletion of comments by operator", () => {
    render(<CommentAccordion postId="123" currentAccountId="42" />);

    expect(screen.getByTestId("can-delete-operator")).toHaveTextContent(
      "false",
    );
  });

  it("updates local unread count after marking as read", async () => {
    const { rerender } = render(
      <CommentAccordion postId="123" currentAccountId="42" unreadCount={3} />,
    );

    expect(screen.getByText(/Unread: 3/)).toBeInTheDocument();

    const toggleButton = screen.getByTestId("accordion-toggle");
    fireEvent.click(toggleButton);

    await waitFor(() => {
      expect(mockMarkCommentsRead).toHaveBeenCalled();
    });

    // After marking as read, unread count should be 0
    rerender(
      <CommentAccordion postId="123" currentAccountId="42" unreadCount={0} />,
    );
    expect(screen.getByText(/Unread: 0/)).toBeInTheDocument();
  });
});
