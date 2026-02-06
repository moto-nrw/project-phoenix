import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { SuggestionCard } from "./suggestion-card";
import type { Suggestion } from "~/lib/suggestions-helpers";

// ============================================================================
// Mocks
// ============================================================================

vi.mock("~/lib/suggestions-api", () => ({
  voteSuggestion: vi.fn(),
  removeVote: vi.fn(),
}));

// ============================================================================
// Test data
// ============================================================================

function createSuggestion(overrides?: Partial<Suggestion>): Suggestion {
  return {
    id: "1",
    title: "PDF-Export Feature",
    description: "Wir brauchen einen PDF-Export für den Vertretungsplan.",
    authorId: "10",
    authorName: "Max Mustermann",
    status: "open",
    score: 5,
    upvotes: 7,
    downvotes: 2,
    commentCount: 0,
    userVote: null,
    createdAt: new Date(Date.now() - 3600000).toISOString(), // 1 hour ago
    updatedAt: new Date(Date.now() - 1800000).toISOString(),
    ...overrides,
  };
}

// ============================================================================
// Tests
// ============================================================================

describe("SuggestionCard", () => {
  const onEdit = vi.fn();
  const onDelete = vi.fn();
  const onVoteChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders title and description", () => {
    render(
      <SuggestionCard
        suggestion={createSuggestion()}
        currentAccountId="10"
        onEdit={onEdit}
        onDelete={onDelete}
        onVoteChange={onVoteChange}
      />,
    );

    expect(screen.getByText("PDF-Export Feature")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Wir brauchen einen PDF-Export für den Vertretungsplan.",
      ),
    ).toBeInTheDocument();
  });

  it("renders author name and initials", () => {
    render(
      <SuggestionCard
        suggestion={createSuggestion()}
        currentAccountId="10"
        onEdit={onEdit}
        onDelete={onDelete}
        onVoteChange={onVoteChange}
      />,
    );

    expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
    expect(screen.getByText("MM")).toBeInTheDocument();
  });

  it("renders status badge", () => {
    render(
      <SuggestionCard
        suggestion={createSuggestion({ status: "planned" })}
        currentAccountId="10"
        onEdit={onEdit}
        onDelete={onDelete}
        onVoteChange={onVoteChange}
      />,
    );

    expect(screen.getByText("Geplant")).toBeInTheDocument();
  });

  it("shows action menu for owner", () => {
    render(
      <SuggestionCard
        suggestion={createSuggestion({ authorId: "10" })}
        currentAccountId="10"
        onEdit={onEdit}
        onDelete={onDelete}
        onVoteChange={onVoteChange}
      />,
    );

    expect(screen.getByLabelText("Aktionen")).toBeInTheDocument();
  });

  it("hides action menu for non-owner", () => {
    render(
      <SuggestionCard
        suggestion={createSuggestion({ authorId: "10" })}
        currentAccountId="999"
        onEdit={onEdit}
        onDelete={onDelete}
        onVoteChange={onVoteChange}
      />,
    );

    expect(screen.queryByLabelText("Aktionen")).not.toBeInTheDocument();
  });

  it("opens menu and calls onEdit when clicking edit", () => {
    const suggestion = createSuggestion({ authorId: "10" });
    render(
      <SuggestionCard
        suggestion={suggestion}
        currentAccountId="10"
        onEdit={onEdit}
        onDelete={onDelete}
        onVoteChange={onVoteChange}
      />,
    );

    fireEvent.click(screen.getByLabelText("Aktionen"));
    fireEvent.click(screen.getByText("Bearbeiten"));

    expect(onEdit).toHaveBeenCalledWith(suggestion);
  });

  it("opens menu and calls onDelete when clicking delete", () => {
    const suggestion = createSuggestion({ authorId: "10" });
    render(
      <SuggestionCard
        suggestion={suggestion}
        currentAccountId="10"
        onEdit={onEdit}
        onDelete={onDelete}
        onVoteChange={onVoteChange}
      />,
    );

    fireEvent.click(screen.getByLabelText("Aktionen"));
    fireEvent.click(screen.getByText("Löschen"));

    expect(onDelete).toHaveBeenCalledWith(suggestion);
  });

  it("closes menu when clicking outside", () => {
    render(
      <SuggestionCard
        suggestion={createSuggestion({ authorId: "10" })}
        currentAccountId="10"
        onEdit={onEdit}
        onDelete={onDelete}
        onVoteChange={onVoteChange}
      />,
    );

    fireEvent.click(screen.getByLabelText("Aktionen"));
    expect(screen.getByText("Bearbeiten")).toBeInTheDocument();

    // Click outside the menu
    fireEvent.mouseDown(document.body);

    expect(screen.queryByText("Bearbeiten")).not.toBeInTheDocument();
  });

  it("renders relative time for recent posts", () => {
    render(
      <SuggestionCard
        suggestion={createSuggestion({
          createdAt: new Date(Date.now() - 5 * 60000).toISOString(), // 5 min ago
        })}
        currentAccountId="10"
        onEdit={onEdit}
        onDelete={onDelete}
        onVoteChange={onVoteChange}
      />,
    );

    expect(screen.getByText("vor 5 Minuten")).toBeInTheDocument();
  });

  it("renders all status styles correctly", () => {
    const statuses = ["open", "planned", "done", "rejected"] as const;
    const labels = ["Offen", "Geplant", "Umgesetzt", "Abgelehnt"];

    for (let i = 0; i < statuses.length; i++) {
      const { unmount } = render(
        <SuggestionCard
          suggestion={createSuggestion({ status: statuses[i] })}
          currentAccountId="10"
          onEdit={onEdit}
          onDelete={onDelete}
          onVoteChange={onVoteChange}
        />,
      );

      expect(screen.getByText(labels[i]!)).toBeInTheDocument();
      unmount();
    }
  });

  it("handles single-name author initials", () => {
    render(
      <SuggestionCard
        suggestion={createSuggestion({ authorName: "Admin" })}
        currentAccountId="10"
        onEdit={onEdit}
        onDelete={onDelete}
        onVoteChange={onVoteChange}
      />,
    );

    expect(screen.getByText("A")).toBeInTheDocument();
  });
});
