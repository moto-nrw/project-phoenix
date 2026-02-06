import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { Session } from "next-auth";
import { mockSessionData } from "~/test/mocks/next-auth";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiGet, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({ auth: mockAuth }));
vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: mockApiPost,
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    return new Response(JSON.stringify({ error: message }), { status: 500 });
  }),
}));

import { GET, POST } from "./route";

function mockValidSession(): void {
  mockAuth.mockResolvedValue(mockSessionData() as ExtendedSession);
}

describe("GET /api/suggestions/[id]/comments", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches comments successfully", async () => {
    mockValidSession();
    const comments = [
      {
        id: 1,
        content: "Great idea!",
        author_name: "User 1",
        author_type: "teacher",
        is_internal: false,
        created_at: "2024-01-01T00:00:00Z",
      },
      {
        id: 2,
        content: "Internal note",
        author_name: "Admin",
        author_type: "operator",
        is_internal: true,
        created_at: "2024-01-02T00:00:00Z",
      },
    ];

    mockApiGet.mockResolvedValue({ status: "success", data: comments });

    const request = new NextRequest(
      "http://localhost:3000/api/suggestions/1/comments",
    );
    const context = { params: Promise.resolve({ id: "1" }) };
    const response = await GET(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as { data?: unknown; error?: string };
    expect(json.data).toEqual(comments);
    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/suggestions/1/comments",
      "test-token",
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/suggestions/1/comments",
    );
    const context = { params: Promise.resolve({ id: "1" }) };
    const response = await GET(request, context);

    expect(response.status).toBe(401);
  });

  it("handles invalid id parameter", async () => {
    mockValidSession();

    const request = new NextRequest(
      "http://localhost:3000/api/suggestions/1/comments",
    );
    const context = {
      params: Promise.resolve({ id: 123 as unknown as string }),
    };
    const response = await GET(request, context);

    expect(response.status).toBe(500);
  });

  it("returns empty array when no comments", async () => {
    mockValidSession();
    mockApiGet.mockResolvedValue({ status: "success", data: [] });

    const request = new NextRequest(
      "http://localhost:3000/api/suggestions/1/comments",
    );
    const context = { params: Promise.resolve({ id: "1" }) };
    const response = await GET(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as { data?: unknown; error?: string };
    expect(json.data).toEqual([]);
  });
});

describe("POST /api/suggestions/[id]/comments", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("creates comment successfully", async () => {
    mockValidSession();
    const commentData = { content: "This is a great suggestion!" };
    mockApiPost.mockResolvedValue({ success: true });

    const request = new NextRequest(
      "http://localhost:3000/api/suggestions/1/comments",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(commentData),
      },
    );
    const context = { params: Promise.resolve({ id: "1" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(200);
    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/suggestions/1/comments",
      "test-token",
      commentData,
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/suggestions/1/comments",
      {
        method: "POST",
        body: JSON.stringify({ content: "Test" }),
      },
    );
    const context = { params: Promise.resolve({ id: "1" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(401);
  });

  it("handles invalid id parameter", async () => {
    mockValidSession();

    const request = new NextRequest(
      "http://localhost:3000/api/suggestions/1/comments",
      {
        method: "POST",
        body: JSON.stringify({ content: "Test" }),
      },
    );
    const context = {
      params: Promise.resolve({ id: 123 as unknown as string }),
    };
    const response = await POST(request, context);

    expect(response.status).toBe(500);
  });

  it("handles empty content", async () => {
    mockValidSession();
    mockApiPost.mockRejectedValue(new Error("Content is required"));

    const request = new NextRequest(
      "http://localhost:3000/api/suggestions/1/comments",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ content: "" }),
      },
    );
    const context = { params: Promise.resolve({ id: "1" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(500);
  });
});
