import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { DELETE } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockApiDelete } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiDelete: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: mockApiDelete,
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

function createMockRequest(path: string): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  return new NextRequest(url);
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

describe("DELETE /api/invitations/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/invitations/123");
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("deletes invitation via backend", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/invitations/123");
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/auth/invitations/123",
      "test-token",
    );
    expect(response.status).toBe(204);
  });

  it("handles numeric ID from params", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/invitations/456");
    const response = await DELETE(request, createMockContext({ id: 456 }));

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/auth/invitations/456",
      "test-token",
    );
    expect(response.status).toBe(204);
  });

  it("handles invitationId fallback param", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/invitations/789");
    const response = await DELETE(
      request,
      createMockContext({ invitationId: "789" }),
    );

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/auth/invitations/789",
      "test-token",
    );
    expect(response.status).toBe(204);
  });

  it("returns 500 when ID is missing", async () => {
    const request = createMockRequest("/api/invitations/");
    const response = await DELETE(request, createMockContext({}));

    // TypeError is caught by wrapper and returns 500
    expect(response.status).toBe(500);
    expect(mockApiDelete).not.toHaveBeenCalled();
  });

  it("returns 500 when ID is invalid type", async () => {
    const request = createMockRequest("/api/invitations/invalid");
    const response = await DELETE(
      request,
      createMockContext({ id: { invalid: true } }),
    );

    // TypeError is caught by wrapper and returns 500
    expect(response.status).toBe(500);
    expect(mockApiDelete).not.toHaveBeenCalled();
  });

  it("forwards backend errors", async () => {
    mockApiDelete.mockRejectedValueOnce(new Error("Not found (404)"));

    const request = createMockRequest("/api/invitations/999");
    const response = await DELETE(request, createMockContext({ id: "999" }));

    expect(response.status).toBe(404);
  });
});
