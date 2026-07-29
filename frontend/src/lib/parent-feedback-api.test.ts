import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createFeedback,
  deleteFeedback,
  fetchFeedback,
  fetchFeedbackSchools,
  parentFeedbackBoardApi,
} from "./parent-feedback-api";

const originalFetch = globalThis.fetch;

interface Call {
  url: string;
  init?: RequestInit;
}

function mockFetch(payload: unknown): Call[] {
  const calls: Call[] = [];
  globalThis.fetch = vi.fn(
    (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
      calls.push({ url: String(input), init });
      return Promise.resolve(
        new Response(JSON.stringify({ success: true, data: payload }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );
    },
  ) as typeof globalThis.fetch;
  return calls;
}

afterEach(() => {
  globalThis.fetch = originalFetch;
  vi.restoreAllMocks();
});

const backendPost = {
  id: "42",
  title: "Push-Nachrichten wären hilfreich",
  description: "Damit ich Krankmeldungen schneller sehe.",
  author_name: "Elternteil",
  is_own: true,
  status: "open" as const,
  score: 3,
  upvotes: 4,
  downvotes: 1,
  comment_count: 2,
  unread_count: 1,
  user_vote: "up" as const,
  created_at: "2026-07-01T08:00:00Z",
  updated_at: "2026-07-01T08:00:00Z",
};

describe("parent feedback board client", () => {
  it("maps an entry without ever inventing an author id", async () => {
    mockFetch([backendPost]);

    const [post] = await fetchFeedback("7");

    // The board is pseudonymous: the only identity is the display name, and
    // ownership comes from the backend rather than an id comparison.
    expect(post?.authorId).toBe("");
    expect(post?.authorName).toBe("Elternteil");
    expect(post?.isOwn).toBe(true);
    expect(post?.commentCount).toBe(2);
    expect(post?.userVote).toBe("up");
  });

  it("scopes every read to the addressed school", async () => {
    const calls = mockFetch([backendPost]);

    await fetchFeedback("7", "newest");

    expect(calls[0]?.url).toContain("school_id=7");
    expect(calls[0]?.url).toContain("sort=newest");
  });

  it("sends the school with writes", async () => {
    const calls = mockFetch(backendPost);

    await createFeedback("7", "Titel", "Beschreibung");

    expect(calls[0]?.init?.method).toBe("POST");
    expect(JSON.parse(String(calls[0]?.init?.body))).toEqual({
      school_id: "7",
      title: "Titel",
      description: "Beschreibung",
    });
  });

  it("passes the school on deletes, where there is no body to carry it", async () => {
    const calls = mockFetch(null);

    await deleteFeedback("7", "42");

    expect(calls[0]?.init?.method).toBe("DELETE");
    expect(calls[0]?.url).toContain("/api/parent/me/feedback/42?school_id=7");
  });

  it("maps the school picker", async () => {
    mockFetch([{ school_id: "7", school_name: "GS Musterstadt" }]);

    const schools = await fetchFeedbackSchools();

    expect(schools).toEqual([{ id: "7", name: "GS Musterstadt" }]);
  });

  describe("board adapter", () => {
    it("votes against the bound school", async () => {
      const calls = mockFetch(backendPost);

      const updated = await parentFeedbackBoardApi("7").vote("42", "up");

      expect(calls[0]?.url).toBe("/api/parent/me/feedback/42/vote");
      expect(JSON.parse(String(calls[0]?.init?.body))).toEqual({
        school_id: "7",
        direction: "up",
      });
      expect(updated.isOwn).toBe(true);
    });

    it("marks a guardian's own comment as theirs and hides the author id", async () => {
      mockFetch([
        {
          id: "9",
          content: "Danke!",
          author_name: "Verfasser",
          author_type: "parent" as const,
          is_own: true,
          created_at: "2026-07-01T09:00:00Z",
        },
      ]);

      const [comment] = await parentFeedbackBoardApi("7").fetchComments("42");

      expect(comment?.authorId).toBe("");
      expect(comment?.authorName).toBe("Verfasser");
      expect(comment?.isOwn).toBe(true);
    });
  });
});
