import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { POST } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: vi.fn(),
  apiPost: mockApiPost,
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    // Extract status code from error message like "Error message (404)"
    const statusMatch = /\((\d+)\)/.exec(message);
    const status = statusMatch ? parseInt(statusMatch[1]!, 10) : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
}));

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(
  path: string,
  options: { method?: string; body?: unknown } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const requestInit: { method: string; body?: string; headers?: HeadersInit } =
    {
      method: options.method ?? "POST",
    };

  if (options.body) {
    requestInit.body = JSON.stringify(options.body);
    requestInit.headers = { "Content-Type": "application/json" };
  }

  return new NextRequest(url, requestInit);
}

function createMockContext(
  params: Record<
    string,
    string | number | Record<string, unknown> | undefined
  > = {},
) {
  return { params: Promise.resolve(params) } as {
    params: Promise<Record<string, string | string[] | undefined>>;
  };
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

// ============================================================================
// Tests
// ============================================================================

describe("POST /api/invitations/[id]/resend", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/invitations/123/resend");
    const response = await POST(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("resends invitation via backend", async () => {
    mockApiPost.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/invitations/123/resend");
    const response = await POST(request, createMockContext({ id: "123" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/auth/invitations/123/resend",
      "test-token",
    );
    // POST handlers wrap null in ApiResponse and return 200, not 204
    expect(response.status).toBe(200);
  });

  it("handles numeric ID from params", async () => {
    mockApiPost.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/invitations/456/resend");
    const response = await POST(request, createMockContext({ id: 456 }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/auth/invitations/456/resend",
      "test-token",
    );
    // POST handlers wrap null in ApiResponse and return 200, not 204
    expect(response.status).toBe(200);
  });

  it("handles invitationId fallback param", async () => {
    mockApiPost.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/invitations/789/resend");
    const response = await POST(
      request,
      createMockContext({ invitationId: "789" }),
    );

    expect(mockApiPost).toHaveBeenCalledWith(
      "/auth/invitations/789/resend",
      "test-token",
    );
    // POST handlers wrap null in ApiResponse and return 200, not 204
    expect(response.status).toBe(200);
  });

  it("returns 500 when ID is missing", async () => {
    const request = createMockRequest("/api/invitations/resend");
    const response = await POST(request, createMockContext({}));

    // TypeError is caught by wrapper and returns 500
    expect(response.status).toBe(500);
    expect(mockApiPost).not.toHaveBeenCalled();
  });

  it("returns 500 when ID is invalid type", async () => {
    const request = createMockRequest("/api/invitations/invalid/resend");
    const response = await POST(
      request,
      createMockContext({ id: { invalid: true } }),
    );

    // TypeError is caught by wrapper and returns 500
    expect(response.status).toBe(500);
    expect(mockApiPost).not.toHaveBeenCalled();
  });

  it("forwards backend errors", async () => {
    mockApiPost.mockRejectedValueOnce(new Error("Not found (404)"));

    const request = createMockRequest("/api/invitations/999/resend");
    const response = await POST(request, createMockContext({ id: "999" }));

    expect(response.status).toBe(404);
  });
});
