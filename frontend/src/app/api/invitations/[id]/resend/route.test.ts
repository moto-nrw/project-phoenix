import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { POST } from "./route";
import {
  type ExtendedSession,
  defaultSession,
} from "~/test/route-test-helpers";

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

vi.mock("~/lib/api-helpers.server", async () => {
  const { mockHandleApiError } =
    await import("~/test/helpers/handle-api-error");
  return {
    apiGet: vi.fn(),
    apiPost: mockApiPost,
    apiPut: vi.fn(),
    apiDelete: vi.fn(),
    handleApiError: vi.fn(mockHandleApiError),
  };
});

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

// ============================================================================
// Tests
// ============================================================================

describe("POST /api/invitations/[id]/resend", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
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
});
