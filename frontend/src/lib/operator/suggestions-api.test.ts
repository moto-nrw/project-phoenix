import { describe, it, expect, vi, beforeEach } from "vitest";

const { mockOperatorFetch } = vi.hoisted(() => ({
  mockOperatorFetch: vi.fn(),
}));

vi.mock("./api-helpers", () => ({
  operatorFetch: mockOperatorFetch,
}));

import { operatorSuggestionsService } from "./suggestions-api";
import type { BackendOperatorSuggestion } from "./suggestions-helpers";

describe("OperatorSuggestionsService", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("fetchAll", () => {
    it("calls correct endpoint without filters", async () => {
      const mockData: BackendOperatorSuggestion[] = [];
      mockOperatorFetch.mockResolvedValue(mockData);

      await operatorSuggestionsService.fetchAll();

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/suggestions",
      );
    });

    it("adds status filter to query string", async () => {
      const mockData: BackendOperatorSuggestion[] = [];
      mockOperatorFetch.mockResolvedValue(mockData);

      await operatorSuggestionsService.fetchAll("planned");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/suggestions?status=planned",
      );
    });

    it("adds search filter to query string", async () => {
      const mockData: BackendOperatorSuggestion[] = [];
      mockOperatorFetch.mockResolvedValue(mockData);

      await operatorSuggestionsService.fetchAll(undefined, "dark mode");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/suggestions?search=dark+mode",
      );
    });

    it("adds both status and search filters", async () => {
      const mockData: BackendOperatorSuggestion[] = [];
      mockOperatorFetch.mockResolvedValue(mockData);

      await operatorSuggestionsService.fetchAll("done", "feature");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/suggestions?status=done&search=feature",
      );
    });

    it("skips status filter when set to 'all'", async () => {
      const mockData: BackendOperatorSuggestion[] = [];
      mockOperatorFetch.mockResolvedValue(mockData);

      await operatorSuggestionsService.fetchAll("all");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/suggestions",
      );
    });

    it("maps response data correctly", async () => {
      const mockData: BackendOperatorSuggestion[] = [
        {
          id: 1,
          title: "Feature",
          description: "Add feature",
          status: "open",
          score: 10,
          upvotes: 10,
          downvotes: 0,
          author_name: "User",
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        },
      ];
      mockOperatorFetch.mockResolvedValue(mockData);

      const result = await operatorSuggestionsService.fetchAll();

      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe("1");
      expect(result[0]?.title).toBe("Feature");
    });
  });

  describe("fetchById", () => {
    it("calls correct endpoint with ID", async () => {
      const mockData: BackendOperatorSuggestion = {
        id: 42,
        title: "Test",
        description: "Desc",
        status: "planned",
        score: 5,
        upvotes: 5,
        downvotes: 0,
        author_name: "Alice",
        created_at: "2024-02-01T00:00:00Z",
        updated_at: "2024-02-01T00:00:00Z",
      };
      mockOperatorFetch.mockResolvedValue(mockData);

      await operatorSuggestionsService.fetchById("42");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/suggestions/42",
      );
    });

    it("maps single suggestion correctly", async () => {
      const mockData: BackendOperatorSuggestion = {
        id: 99,
        title: "Bug Fix",
        description: "Fix bug",
        status: "in_progress",
        score: 8,
        upvotes: 10,
        downvotes: 2,
        author_name: "Bob",
        created_at: "2024-03-01T00:00:00Z",
        updated_at: "2024-03-02T00:00:00Z",
      };
      mockOperatorFetch.mockResolvedValue(mockData);

      const result = await operatorSuggestionsService.fetchById("99");

      expect(result.id).toBe("99");
      expect(result.title).toBe("Bug Fix");
      expect(result.status).toBe("in_progress");
    });
  });

  describe("updateStatus", () => {
    it("calls PUT endpoint with status", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await operatorSuggestionsService.updateStatus("42", "done");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/suggestions/42/status",
        {
          method: "PUT",
          body: { status: "done" },
        },
      );
    });
  });

  describe("addComment", () => {
    it("calls POST endpoint with comment data", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await operatorSuggestionsService.addComment("42", "Great idea!", false);

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/suggestions/42/comments",
        {
          method: "POST",
          body: { content: "Great idea!", is_internal: false },
        },
      );
    });

    it("sends internal comment correctly", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await operatorSuggestionsService.addComment("42", "Internal note", true);

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/suggestions/42/comments",
        {
          method: "POST",
          body: { content: "Internal note", is_internal: true },
        },
      );
    });
  });

  describe("deleteComment", () => {
    it("calls DELETE endpoint with IDs", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await operatorSuggestionsService.deleteComment("42", "100");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/suggestions/42/comments/100",
        { method: "DELETE" },
      );
    });
  });

  describe("markCommentsRead", () => {
    it("calls POST endpoint to mark comments read", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await operatorSuggestionsService.markCommentsRead("42");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/suggestions/42/comments/read",
        { method: "POST" },
      );
    });
  });

  describe("fetchUnreadCount", () => {
    it("calls correct endpoint and returns count", async () => {
      mockOperatorFetch.mockResolvedValue({ unread_count: 5 });

      const result = await operatorSuggestionsService.fetchUnreadCount();

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/suggestions/unread-count",
      );
      expect(result).toBe(5);
    });

    it("returns zero when no unread", async () => {
      mockOperatorFetch.mockResolvedValue({ unread_count: 0 });

      const result = await operatorSuggestionsService.fetchUnreadCount();

      expect(result).toBe(0);
    });
  });

  describe("markPostViewed", () => {
    it("calls POST endpoint to mark post viewed", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await operatorSuggestionsService.markPostViewed("42");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/suggestions/42/view",
        { method: "POST" },
      );
    });
  });

  describe("fetchUnviewedCount", () => {
    it("calls correct endpoint and returns count", async () => {
      mockOperatorFetch.mockResolvedValue({ unviewed_count: 3 });

      const result = await operatorSuggestionsService.fetchUnviewedCount();

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/suggestions/unviewed-count",
      );
      expect(result).toBe(3);
    });

    it("returns zero when no unviewed", async () => {
      mockOperatorFetch.mockResolvedValue({ unviewed_count: 0 });

      const result = await operatorSuggestionsService.fetchUnviewedCount();

      expect(result).toBe(0);
    });
  });
});
