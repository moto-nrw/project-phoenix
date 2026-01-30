import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { mockSessionData } from "~/test/mocks/next-auth";

// ============================================================================
// Mocks
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiPost, mockApiDelete } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPost: vi.fn(),
  mockApiDelete: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiPost: mockApiPost,
  apiDelete: mockApiDelete,
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    return new Response(JSON.stringify({ error: message }), { status: 500 });
  }),
}));

import { POST, DELETE } from "./route";

// ============================================================================
// Helpers
// ============================================================================

function createMockRequest(
  path: string,
  options: { method?: string; body?: unknown } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const init: { method: string; body?: string; headers?: HeadersInit } = {
    method: options.method ?? "GET",
  };
  if (options.body) {
    init.body = JSON.stringify(options.body);
    init.headers = { "Content-Type": "application/json" };
  }
  return new NextRequest(url, init);
}

function mockValidSession(): void {
  mockAuth.mockResolvedValue(mockSessionData() as ExtendedSession);
}

// ============================================================================
// Tests
// ============================================================================

describe("POST /api/suggestions/[id]/vote", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("casts an upvote", async () => {
    mockValidSession();
    mockApiPost.mockResolvedValue({
      data: { id: 1, user_vote: "up", score: 1 },
    });

    const req = createMockRequest("/api/suggestions/1/vote", {
      method: "POST",
      body: { direction: "up" },
    });
    const response = await POST(req, {
      params: Promise.resolve({ id: "1" }),
    });
    const json = (await response.json()) as { success: boolean };

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/suggestions/1/vote",
      "test-token",
      { direction: "up" },
    );
    expect(json.success).toBe(true);
  });

  it("casts a downvote", async () => {
    mockValidSession();
    mockApiPost.mockResolvedValue({
      data: { id: 1, user_vote: "down", score: -1 },
    });

    const req = createMockRequest("/api/suggestions/1/vote", {
      method: "POST",
      body: { direction: "down" },
    });
    const response = await POST(req, {
      params: Promise.resolve({ id: "1" }),
    });

    expect(response.status).toBe(200);
  });

  it("returns error for invalid id", async () => {
    mockValidSession();

    const req = createMockRequest("/api/suggestions/bad/vote", {
      method: "POST",
      body: { direction: "up" },
    });
    const response = await POST(req, {
      params: Promise.resolve({ id: ["a", "b"] }),
    });

    expect(response.status).toBe(500);
  });
});

describe("DELETE /api/suggestions/[id]/vote", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("removes a vote", async () => {
    mockValidSession();
    mockApiDelete.mockResolvedValue({
      data: { id: 1, user_vote: null, score: 0 },
    });

    const req = createMockRequest("/api/suggestions/1/vote", {
      method: "DELETE",
    });
    const response = await DELETE(req, {
      params: Promise.resolve({ id: "1" }),
    });
    const json = (await response.json()) as { success: boolean };

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/suggestions/1/vote",
      "test-token",
    );
    expect(json.success).toBe(true);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const req = createMockRequest("/api/suggestions/1/vote", {
      method: "DELETE",
    });
    const response = await DELETE(req, {
      params: Promise.resolve({ id: "1" }),
    });

    expect(response.status).toBe(401);
  });
});
