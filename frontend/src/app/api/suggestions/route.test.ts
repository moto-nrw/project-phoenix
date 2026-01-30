import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";

// ============================================================================
// Mocks
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiGet, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

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
  mockAuth.mockResolvedValue({
    user: { token: "test-token" },
    expires: new Date(Date.now() + 3600000).toISOString(),
  } as ExtendedSession);
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/suggestions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns suggestions with default sort", async () => {
    mockValidSession();
    mockApiGet.mockResolvedValue({
      data: [{ id: 1, title: "Test" }],
    });

    const req = createMockRequest("/api/suggestions");
    const response = await GET(req, { params: Promise.resolve({}) });
    const json = (await response.json()) as { success: boolean };

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/suggestions?sort=score",
      "test-token",
    );
    expect(json.success).toBe(true);
  });

  it("passes sort parameter from query string", async () => {
    mockValidSession();
    mockApiGet.mockResolvedValue({ data: [] });

    const req = createMockRequest("/api/suggestions?sort=newest");
    await GET(req, { params: Promise.resolve({}) });

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/suggestions?sort=newest",
      "test-token",
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const req = createMockRequest("/api/suggestions");
    const response = await GET(req, { params: Promise.resolve({}) });

    expect(response.status).toBe(401);
  });
});

describe("POST /api/suggestions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("creates suggestion and returns response", async () => {
    mockValidSession();
    mockApiPost.mockResolvedValue({
      data: { id: 1, title: "New", description: "Desc" },
    });

    const req = createMockRequest("/api/suggestions", {
      method: "POST",
      body: { title: "New", description: "Desc" },
    });
    const response = await POST(req, { params: Promise.resolve({}) });
    const json = (await response.json()) as { success: boolean };

    expect(mockApiPost).toHaveBeenCalledWith("/api/suggestions", "test-token", {
      title: "New",
      description: "Desc",
    });
    expect(json.success).toBe(true);
  });
});
