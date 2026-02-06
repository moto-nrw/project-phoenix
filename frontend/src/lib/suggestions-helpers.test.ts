import { describe, it, expect } from "vitest";
import type { BackendSuggestion } from "./suggestions-helpers";
import {
  mapSuggestionResponse,
  STATUS_LABELS,
  STATUS_STYLES,
  SORT_LABELS,
} from "./suggestions-helpers";

const sampleBackendSuggestion: BackendSuggestion = {
  id: 42,
  title: "PDF-Export für Vertretungsplan",
  description: "Wir brauchen einen PDF-Export.",
  author_id: 7,
  author_name: "Max Mustermann",
  status: "open",
  score: 5,
  upvotes: 8,
  downvotes: 3,
  comment_count: 2,
  user_vote: "up",
  created_at: "2025-01-15T10:00:00Z",
  updated_at: "2025-01-16T12:30:00Z",
};

describe("mapSuggestionResponse", () => {
  it("maps backend fields to frontend camelCase with string IDs", () => {
    const result = mapSuggestionResponse(sampleBackendSuggestion);

    expect(result).toEqual({
      id: "42",
      title: "PDF-Export für Vertretungsplan",
      description: "Wir brauchen einen PDF-Export.",
      authorId: "7",
      authorName: "Max Mustermann",
      status: "open",
      score: 5,
      upvotes: 8,
      downvotes: 3,
      commentCount: 2,
      userVote: "up",
      createdAt: "2025-01-15T10:00:00Z",
      updatedAt: "2025-01-16T12:30:00Z",
    });
  });

  it("handles null user_vote", () => {
    const data: BackendSuggestion = {
      ...sampleBackendSuggestion,
      user_vote: null,
    };
    const result = mapSuggestionResponse(data);
    expect(result.userVote).toBeNull();
  });

  it("handles down vote direction", () => {
    const data: BackendSuggestion = {
      ...sampleBackendSuggestion,
      user_vote: "down",
    };
    const result = mapSuggestionResponse(data);
    expect(result.userVote).toBe("down");
  });

  it("converts numeric IDs to strings", () => {
    const data: BackendSuggestion = {
      ...sampleBackendSuggestion,
      id: 999,
      author_id: 123,
    };
    const result = mapSuggestionResponse(data);
    expect(result.id).toBe("999");
    expect(result.authorId).toBe("123");
  });

  it("preserves all status values", () => {
    const statuses = ["open", "planned", "done", "rejected"] as const;
    for (const status of statuses) {
      const data: BackendSuggestion = { ...sampleBackendSuggestion, status };
      expect(mapSuggestionResponse(data).status).toBe(status);
    }
  });
});

describe("STATUS_LABELS", () => {
  it("has German labels for all statuses", () => {
    expect(STATUS_LABELS.open).toBe("Offen");
    expect(STATUS_LABELS.planned).toBe("Geplant");
    expect(STATUS_LABELS.done).toBe("Umgesetzt");
    expect(STATUS_LABELS.rejected).toBe("Abgelehnt");
  });
});

describe("STATUS_STYLES", () => {
  it("has Tailwind classes for all statuses", () => {
    expect(STATUS_STYLES.open).toContain("bg-gray");
    expect(STATUS_STYLES.planned).toContain("bg-blue");
    expect(STATUS_STYLES.done).toContain("bg-green");
    expect(STATUS_STYLES.rejected).toContain("bg-red");
  });
});

describe("SORT_LABELS", () => {
  it("has German labels for all sort options", () => {
    expect(SORT_LABELS.score).toBe("Beliebteste");
    expect(SORT_LABELS.newest).toBe("Neueste");
    expect(SORT_LABELS.status).toBe("Nach Status");
  });
});
