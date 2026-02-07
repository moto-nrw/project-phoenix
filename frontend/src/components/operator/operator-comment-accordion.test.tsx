import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { OperatorCommentAccordion } from "./operator-comment-accordion";
import type { OperatorComment } from "~/lib/operator/suggestions-helpers";

// Mock the suggestions service
const { mockOperatorSuggestionsService } = vi.hoisted(() => ({
  mockOperatorSuggestionsService: {
    fetchById: vi.fn(),
    addComment: vi.fn(),
    deleteComment: vi.fn(),
    markPostViewed: vi.fn(),
    markCommentsRead: vi.fn(),
  },
}));

vi.mock("~/lib/operator/suggestions-api", () => ({
  operatorSuggestionsService: mockOperatorSuggestionsService,
}));

// Mock BaseCommentAccordion
interface MockBaseCommentAccordionProps {
  postId: string;
  onOpen?: (postId: string) => void;
  onAfterCreate?: (postId: string, reload: () => Promise<void>) => void;
  [key: string]: unknown;
}

const { MockBaseCommentAccordion } = vi.hoisted(() => ({
  MockBaseCommentAccordion: vi.fn(
    ({ postId, onOpen, onAfterCreate }: MockBaseCommentAccordionProps) => {
      return (
        <div data-testid="base-comment-accordion">
          <button onClick={() => onOpen?.(postId)} data-testid="open-accordion">
            Open
          </button>
          <button
            onClick={() =>
              onAfterCreate?.(postId, async () => {
                /* mock reload */
              })
            }
            data-testid="after-create"
          >
            After Create
          </button>
        </div>
      );
    },
  ),
}));

vi.mock("~/components/shared/base-comment-accordion", () => ({
  BaseCommentAccordion: MockBaseCommentAccordion,
}));

describe("OperatorCommentAccordion", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders BaseCommentAccordion with correct props", () => {
    render(
      <OperatorCommentAccordion
        postId="42"
        commentCount={5}
        unreadCount={2}
        isNew={true}
      />,
    );

    expect(screen.getByTestId("base-comment-accordion")).toBeInTheDocument();
    expect(MockBaseCommentAccordion).toHaveBeenCalled();

    const call = MockBaseCommentAccordion.mock.calls[0];
    if (!call) throw new Error("Expected mock to be called");
    const props = call[0];
    expect(props).toMatchObject({
      postId: "42",
      commentCount: 5,
      unreadCount: 2,
    });
  });

  it("loads comments from API", async () => {
    const mockComments: OperatorComment[] = [
      {
        id: "1",
        content: "Test comment",
        authorId: "42",
        authorName: "User",
        authorType: "user",
        createdAt: "2024-01-01T00:00:00Z",
      },
    ];

    mockOperatorSuggestionsService.fetchById.mockResolvedValue({
      operatorComments: mockComments,
    });

    render(<OperatorCommentAccordion postId="42" />);

    const props = MockBaseCommentAccordion.mock.calls[0]?.[0] as {
      loadComments?: (postId: string) => Promise<OperatorComment[]>;
    };
    const result = await props?.loadComments?.("42");

    expect(mockOperatorSuggestionsService.fetchById).toHaveBeenCalledWith("42");
    expect(result).toEqual(mockComments);
  });

  it("creates comment via API", async () => {
    mockOperatorSuggestionsService.addComment.mockResolvedValue(undefined);

    render(<OperatorCommentAccordion postId="42" />);

    const props = MockBaseCommentAccordion.mock.calls[0]?.[0] as {
      createComment?: (postId: string, content: string) => Promise<void>;
    };
    await props?.createComment?.("42", "New comment");

    expect(mockOperatorSuggestionsService.addComment).toHaveBeenCalledWith(
      "42",
      "New comment",
    );
  });

  it("deletes comment via API", async () => {
    mockOperatorSuggestionsService.deleteComment.mockResolvedValue(undefined);

    render(<OperatorCommentAccordion postId="42" />);

    const props = MockBaseCommentAccordion.mock.calls[0]?.[0] as {
      deleteComment?: (postId: string, commentId: string) => Promise<void>;
    };
    await props?.deleteComment?.("42", "100");

    expect(mockOperatorSuggestionsService.deleteComment).toHaveBeenCalledWith(
      "42",
      "100",
    );
  });

  it("marks post viewed when opening if isNew", async () => {
    mockOperatorSuggestionsService.markPostViewed.mockResolvedValue(undefined);

    const { getByTestId } = render(
      <OperatorCommentAccordion postId="42" isNew={true} />,
    );

    const openButton = getByTestId("open-accordion");
    openButton.click();

    await waitFor(() => {
      expect(
        mockOperatorSuggestionsService.markPostViewed,
      ).toHaveBeenCalledWith("42");
    });
  });

  it("does not mark post viewed if not isNew", async () => {
    mockOperatorSuggestionsService.markPostViewed.mockResolvedValue(undefined);

    const { getByTestId } = render(
      <OperatorCommentAccordion postId="42" isNew={false} />,
    );

    const openButton = getByTestId("open-accordion");
    openButton.click();

    await waitFor(() => {
      expect(
        mockOperatorSuggestionsService.markPostViewed,
      ).not.toHaveBeenCalled();
    });
  });

  it("marks comments read when opening if unreadCount > 0", async () => {
    mockOperatorSuggestionsService.markCommentsRead.mockResolvedValue(
      undefined,
    );

    const { getByTestId } = render(
      <OperatorCommentAccordion postId="42" unreadCount={3} />,
    );

    const openButton = getByTestId("open-accordion");
    openButton.click();

    await waitFor(() => {
      expect(
        mockOperatorSuggestionsService.markCommentsRead,
      ).toHaveBeenCalledWith("42");
    });
  });

  it("does not mark comments read if unreadCount is 0", async () => {
    mockOperatorSuggestionsService.markCommentsRead.mockResolvedValue(
      undefined,
    );

    const { getByTestId } = render(
      <OperatorCommentAccordion postId="42" unreadCount={0} />,
    );

    const openButton = getByTestId("open-accordion");
    openButton.click();

    await waitFor(() => {
      expect(
        mockOperatorSuggestionsService.markCommentsRead,
      ).not.toHaveBeenCalled();
    });
  });

  it("dispatches custom event when marking post viewed", async () => {
    mockOperatorSuggestionsService.markPostViewed.mockResolvedValue(undefined);
    const eventListener = vi.fn();
    window.addEventListener(
      "operator-suggestions-unviewed-refresh",
      eventListener,
    );

    const { getByTestId } = render(
      <OperatorCommentAccordion postId="42" isNew={true} />,
    );

    const openButton = getByTestId("open-accordion");
    openButton.click();

    await waitFor(() => {
      expect(eventListener).toHaveBeenCalled();
    });

    window.removeEventListener(
      "operator-suggestions-unviewed-refresh",
      eventListener,
    );
  });

  it("dispatches custom event when marking comments read", async () => {
    mockOperatorSuggestionsService.markCommentsRead.mockResolvedValue(
      undefined,
    );
    const eventListener = vi.fn();
    window.addEventListener(
      "operator-suggestions-unread-refresh",
      eventListener,
    );

    const { getByTestId } = render(
      <OperatorCommentAccordion postId="42" unreadCount={2} />,
    );

    const openButton = getByTestId("open-accordion");
    openButton.click();

    await waitFor(() => {
      expect(eventListener).toHaveBeenCalled();
    });

    window.removeEventListener(
      "operator-suggestions-unread-refresh",
      eventListener,
    );
  });

  it("marks comments read after creating comment", async () => {
    mockOperatorSuggestionsService.markCommentsRead.mockResolvedValue(
      undefined,
    );

    const { getByTestId } = render(<OperatorCommentAccordion postId="42" />);

    const afterCreateButton = getByTestId("after-create");
    afterCreateButton.click();

    await waitFor(() => {
      expect(
        mockOperatorSuggestionsService.markCommentsRead,
      ).toHaveBeenCalledWith("42");
    });
  });
});
