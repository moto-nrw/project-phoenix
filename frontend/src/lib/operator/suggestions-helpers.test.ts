import { describe, it, expect } from "vitest";
import {
  mapOperatorComment,
  mapOperatorSuggestion,
  OPERATOR_STATUS_LABELS,
  OPERATOR_STATUS_STYLES,
  OPERATOR_STATUS_DOT_COLORS,
} from "./suggestions-helpers";
import type {
  BackendOperatorComment,
  BackendOperatorSuggestion,
} from "./suggestions-helpers";

describe("mapOperatorComment", () => {
  it("maps backend comment to frontend type", () => {
    const backendComment: BackendOperatorComment = {
      id: 42,
      content: "Test comment",
      author_name: "John Doe",
      author_type: "operator",
      created_at: "2024-01-15T10:30:00Z",
    };

    const result = mapOperatorComment(backendComment);

    expect(result.id).toBe("42");
    expect(result.content).toBe("Test comment");
    expect(result.authorName).toBe("John Doe");
    expect(result.authorType).toBe("operator");
    expect(result.createdAt).toBe("2024-01-15T10:30:00Z");
  });

  it("maps user comment correctly", () => {
    const backendComment: BackendOperatorComment = {
      id: 123,
      content: "User feedback",
      author_name: "Jane Smith",
      author_type: "user",
      created_at: "2024-02-20T14:45:00Z",
    };

    const result = mapOperatorComment(backendComment);

    expect(result.id).toBe("123");
    expect(result.authorType).toBe("user");
  });
});

describe("mapOperatorSuggestion", () => {
  it("maps backend suggestion with all fields", () => {
    const backendSuggestion: BackendOperatorSuggestion = {
      id: 99,
      title: "Feature Request",
      description: "Add dark mode",
      status: "planned",
      score: 15,
      upvotes: 20,
      downvotes: 5,
      author_name: "Alice",
      created_at: "2024-03-01T09:00:00Z",
      updated_at: "2024-03-02T10:00:00Z",
      comment_count: 3,
      unread_count: 1,
      is_new: true,
      operator_comments: [
        {
          id: 1,
          content: "We'll look into this",
          author_name: "Operator",
          author_type: "operator",
          created_at: "2024-03-01T10:00:00Z",
        },
      ],
    };

    const result = mapOperatorSuggestion(backendSuggestion);

    expect(result.id).toBe("99");
    expect(result.title).toBe("Feature Request");
    expect(result.description).toBe("Add dark mode");
    expect(result.status).toBe("planned");
    expect(result.score).toBe(15);
    expect(result.upvotes).toBe(20);
    expect(result.downvotes).toBe(5);
    expect(result.authorName).toBe("Alice");
    expect(result.createdAt).toBe("2024-03-01T09:00:00Z");
    expect(result.updatedAt).toBe("2024-03-02T10:00:00Z");
    expect(result.commentCount).toBe(3);
    expect(result.unreadCount).toBe(1);
    expect(result.isNew).toBe(true);
    expect(result.operatorComments).toHaveLength(1);
    expect(result.operatorComments[0]?.id).toBe("1");
  });

  it("maps suggestion with missing optional fields", () => {
    const backendSuggestion: BackendOperatorSuggestion = {
      id: 100,
      title: "Bug Report",
      description: "Fix login",
      status: "open",
      score: 0,
      upvotes: 0,
      downvotes: 0,
      author_name: "Bob",
      created_at: "2024-04-01T08:00:00Z",
      updated_at: "2024-04-01T08:00:00Z",
    };

    const result = mapOperatorSuggestion(backendSuggestion);

    expect(result.id).toBe("100");
    expect(result.commentCount).toBe(0);
    expect(result.unreadCount).toBe(0);
    expect(result.isNew).toBe(false);
    expect(result.operatorComments).toEqual([]);
  });

  it("maps nested operator comments correctly", () => {
    const backendSuggestion: BackendOperatorSuggestion = {
      id: 101,
      title: "Test",
      description: "Test desc",
      status: "done",
      score: 10,
      upvotes: 10,
      downvotes: 0,
      author_name: "Charlie",
      created_at: "2024-05-01T00:00:00Z",
      updated_at: "2024-05-02T00:00:00Z",
      operator_comments: [
        {
          id: 10,
          content: "Comment 1",
          author_name: "Op1",
          author_type: "operator",
          created_at: "2024-05-01T01:00:00Z",
        },
        {
          id: 20,
          content: "Comment 2",
          author_name: "User1",
          author_type: "user",
          created_at: "2024-05-01T02:00:00Z",
        },
      ],
    };

    const result = mapOperatorSuggestion(backendSuggestion);

    expect(result.operatorComments).toHaveLength(2);
    expect(result.operatorComments[0]?.id).toBe("10");
    expect(result.operatorComments[0]?.authorType).toBe("operator");
    expect(result.operatorComments[1]?.id).toBe("20");
    expect(result.operatorComments[1]?.authorType).toBe("user");
  });
});

describe("OPERATOR_STATUS_LABELS", () => {
  it("contains all status labels", () => {
    expect(OPERATOR_STATUS_LABELS.open).toBe("Offen");
    expect(OPERATOR_STATUS_LABELS.planned).toBe("Geplant");
    expect(OPERATOR_STATUS_LABELS.in_progress).toBe("In Bearbeitung");
    expect(OPERATOR_STATUS_LABELS.done).toBe("Umgesetzt");
    expect(OPERATOR_STATUS_LABELS.rejected).toBe("Abgelehnt");
    expect(OPERATOR_STATUS_LABELS.need_info).toBe("Rückfrage");
  });

  it("has entries for all status values", () => {
    const statuses = [
      "open",
      "planned",
      "in_progress",
      "done",
      "rejected",
      "need_info",
    ];
    statuses.forEach((status) => {
      expect(OPERATOR_STATUS_LABELS).toHaveProperty(status);
    });
  });
});

describe("OPERATOR_STATUS_STYLES", () => {
  it("contains style classes for all statuses", () => {
    expect(OPERATOR_STATUS_STYLES.open).toContain("bg-white");
    expect(OPERATOR_STATUS_STYLES.planned).toContain("border-[#5080D8]");
    expect(OPERATOR_STATUS_STYLES.in_progress).toContain("border-[#F78C10]");
    expect(OPERATOR_STATUS_STYLES.done).toContain("border-[#83CD2D]");
    expect(OPERATOR_STATUS_STYLES.rejected).toContain("border-[#FF3130]");
    expect(OPERATOR_STATUS_STYLES.need_info).toContain("border-purple-500");
  });
});

describe("OPERATOR_STATUS_DOT_COLORS", () => {
  it("contains dot colors for all statuses", () => {
    expect(OPERATOR_STATUS_DOT_COLORS.open).toBe("bg-gray-400");
    expect(OPERATOR_STATUS_DOT_COLORS.planned).toBe("bg-[#5080D8]");
    expect(OPERATOR_STATUS_DOT_COLORS.in_progress).toBe("bg-[#F78C10]");
    expect(OPERATOR_STATUS_DOT_COLORS.done).toBe("bg-[#83CD2D]");
    expect(OPERATOR_STATUS_DOT_COLORS.rejected).toBe("bg-[#FF3130]");
    expect(OPERATOR_STATUS_DOT_COLORS.need_info).toBe("bg-purple-500");
  });
});
