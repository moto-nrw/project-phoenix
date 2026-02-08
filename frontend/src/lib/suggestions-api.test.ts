import { describe, it, expect, vi, beforeEach } from "vitest";
import type { BackendSuggestion } from "./suggestions-helpers";

// ============================================================================
// Mocks
// ============================================================================

const mockAuthFetch = vi.hoisted(() => vi.fn());

vi.mock("./api-helpers", () => ({
  authFetch: mockAuthFetch,
}));

// Import after mocks are set up
import {
  fetchSuggestions,
  createSuggestion,
  updateSuggestion,
  deleteSuggestion,
  voteSuggestion,
  removeVote,
} from "./suggestions-api";

// ============================================================================
// Test data
// ============================================================================

const backendSuggestion: BackendSuggestion = {
  id: 1,
  title: "Test Suggestion",
  description: "Test description",
  author_id: 10,
  author_name: "Test Author",
  status: "open",
  score: 3,
  upvotes: 4,
  downvotes: 1,
  comment_count: 0,
  unread_count: 0,
  user_vote: null,
  created_at: "2025-01-01T00:00:00Z",
  updated_at: "2025-01-01T00:00:00Z",
};

// ============================================================================
// Tests
// ============================================================================

describe("suggestions-api", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("fetchSuggestions", () => {
    it("fetches suggestions with default sort", async () => {
      mockAuthFetch.mockResolvedValue({ data: [backendSuggestion] });

      const result = await fetchSuggestions();

      expect(mockAuthFetch).toHaveBeenCalledWith("/api/suggestions?sort=score");
      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe("1");
    });

    it("passes sort parameter", async () => {
      mockAuthFetch.mockResolvedValue({ data: [backendSuggestion] });

      await fetchSuggestions("newest");

      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/suggestions?sort=newest",
      );
    });

    it("maps all items in the response", async () => {
      mockAuthFetch.mockResolvedValue({
        data: [backendSuggestion, { ...backendSuggestion, id: 2 }],
      });

      const result = await fetchSuggestions();
      expect(result).toHaveLength(2);
      expect(result[1]?.id).toBe("2");
    });
  });

  describe("createSuggestion", () => {
    it("posts new suggestion and maps response", async () => {
      mockAuthFetch.mockResolvedValue({ data: backendSuggestion });

      const result = await createSuggestion({
        title: "New",
        description: "Desc",
      });

      expect(mockAuthFetch).toHaveBeenCalledWith("/api/suggestions", {
        method: "POST",
        body: { title: "New", description: "Desc" },
      });
      expect(result.id).toBe("1");
      expect(result.title).toBe("Test Suggestion");
    });
  });

  describe("updateSuggestion", () => {
    it("puts updated suggestion and maps response", async () => {
      mockAuthFetch.mockResolvedValue({ data: backendSuggestion });

      const result = await updateSuggestion("1", {
        title: "Updated",
        description: "Updated desc",
      });

      expect(mockAuthFetch).toHaveBeenCalledWith("/api/suggestions/1", {
        method: "PUT",
        body: { title: "Updated", description: "Updated desc" },
      });
      expect(result.id).toBe("1");
    });
  });

  describe("deleteSuggestion", () => {
    it("sends DELETE request", async () => {
      mockAuthFetch.mockResolvedValue(undefined);

      await deleteSuggestion("42");

      expect(mockAuthFetch).toHaveBeenCalledWith("/api/suggestions/42", {
        method: "DELETE",
      });
    });
  });

  describe("voteSuggestion", () => {
    it("posts vote and maps response", async () => {
      const voted = {
        ...backendSuggestion,
        user_vote: "up" as const,
        score: 4,
      };
      mockAuthFetch.mockResolvedValue({ data: voted });

      const result = await voteSuggestion("1", "up");

      expect(mockAuthFetch).toHaveBeenCalledWith("/api/suggestions/1/vote", {
        method: "POST",
        body: { direction: "up" },
      });
      expect(result.userVote).toBe("up");
    });

    it("supports down vote direction", async () => {
      const voted = { ...backendSuggestion, user_vote: "down" as const };
      mockAuthFetch.mockResolvedValue({ data: voted });

      const result = await voteSuggestion("1", "down");

      expect(result.userVote).toBe("down");
    });
  });

  describe("removeVote", () => {
    it("sends DELETE to vote endpoint and maps response", async () => {
      mockAuthFetch.mockResolvedValue({ data: backendSuggestion });

      const result = await removeVote("5");

      expect(mockAuthFetch).toHaveBeenCalledWith("/api/suggestions/5/vote", {
        method: "DELETE",
      });
      expect(result.userVote).toBeNull();
    });
  });
});
