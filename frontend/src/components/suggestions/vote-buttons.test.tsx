import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { VoteButtons } from "./vote-buttons";
import type { Suggestion } from "~/lib/suggestions-helpers";

// ============================================================================
// Mocks
// ============================================================================

const mockVoteSuggestion = vi.hoisted(() => vi.fn());
const mockRemoveVote = vi.hoisted(() => vi.fn());

vi.mock("~/lib/suggestions-api", () => ({
  voteSuggestion: mockVoteSuggestion,
  removeVote: mockRemoveVote,
}));

// ============================================================================
// Test data
// ============================================================================

function createSuggestion(overrides?: Partial<Suggestion>): Suggestion {
  return {
    id: "1",
    title: "Test",
    description: "Test desc",
    authorId: "10",
    authorName: "Author",
    status: "open",
    score: 3,
    upvotes: 4,
    downvotes: 1,
    commentCount: 0,
    userVote: null,
    createdAt: "2025-01-01T00:00:00Z",
    updatedAt: "2025-01-01T00:00:00Z",
    ...overrides,
  };
}

// ============================================================================
// Tests
// ============================================================================

describe("VoteButtons", () => {
  const onVoteChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders upvote and downvote counts", () => {
    render(
      <VoteButtons
        suggestion={createSuggestion({ upvotes: 8, downvotes: 2 })}
        onVoteChange={onVoteChange}
      />,
    );

    expect(screen.getByText("8")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
  });

  it("renders upvote and downvote buttons with aria labels", () => {
    render(
      <VoteButtons
        suggestion={createSuggestion()}
        onVoteChange={onVoteChange}
      />,
    );

    expect(screen.getByLabelText("Positiv bewerten")).toBeInTheDocument();
    expect(screen.getByLabelText("Negativ bewerten")).toBeInTheDocument();
  });

  it("marks upvote button as pressed when user voted up", () => {
    render(
      <VoteButtons
        suggestion={createSuggestion({ userVote: "up" })}
        onVoteChange={onVoteChange}
      />,
    );

    expect(screen.getByLabelText("Positiv bewerten")).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByLabelText("Negativ bewerten")).toHaveAttribute(
      "aria-pressed",
      "false",
    );
  });

  it("marks downvote button as pressed when user voted down", () => {
    render(
      <VoteButtons
        suggestion={createSuggestion({ userVote: "down" })}
        onVoteChange={onVoteChange}
      />,
    );

    expect(screen.getByLabelText("Positiv bewerten")).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    expect(screen.getByLabelText("Negativ bewerten")).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("calls voteSuggestion on upvote click when no active vote", async () => {
    const updated = createSuggestion({ userVote: "up", upvotes: 5, score: 4 });
    mockVoteSuggestion.mockResolvedValue(updated);

    render(
      <VoteButtons
        suggestion={createSuggestion()}
        onVoteChange={onVoteChange}
      />,
    );

    fireEvent.click(screen.getByLabelText("Positiv bewerten"));

    // Wait for async call
    await vi.waitFor(() => {
      expect(mockVoteSuggestion).toHaveBeenCalledWith("1", "up");
    });
  });

  it("calls removeVote when clicking active vote (toggle off)", async () => {
    const updated = createSuggestion({ userVote: null, upvotes: 3, score: 2 });
    mockRemoveVote.mockResolvedValue(updated);

    render(
      <VoteButtons
        suggestion={createSuggestion({ userVote: "up" })}
        onVoteChange={onVoteChange}
      />,
    );

    fireEvent.click(screen.getByLabelText("Positiv bewerten"));

    await vi.waitFor(() => {
      expect(mockRemoveVote).toHaveBeenCalledWith("1");
    });
  });

  it("calls voteSuggestion when changing vote direction", async () => {
    const updated = createSuggestion({
      userVote: "down",
      downvotes: 2,
      score: 1,
    });
    mockVoteSuggestion.mockResolvedValue(updated);

    render(
      <VoteButtons
        suggestion={createSuggestion({ userVote: "up" })}
        onVoteChange={onVoteChange}
      />,
    );

    fireEvent.click(screen.getByLabelText("Negativ bewerten"));

    await vi.waitFor(() => {
      expect(mockVoteSuggestion).toHaveBeenCalledWith("1", "down");
    });
  });

  it("reverts optimistic state on error", async () => {
    mockVoteSuggestion.mockRejectedValue(new Error("Network error"));

    render(
      <VoteButtons
        suggestion={createSuggestion({ upvotes: 4, downvotes: 1 })}
        onVoteChange={onVoteChange}
      />,
    );

    fireEvent.click(screen.getByLabelText("Positiv bewerten"));

    // After error, counts should revert
    await vi.waitFor(() => {
      expect(screen.getByText("4")).toBeInTheDocument();
      expect(screen.getByText("1")).toBeInTheDocument();
    });
  });
});
