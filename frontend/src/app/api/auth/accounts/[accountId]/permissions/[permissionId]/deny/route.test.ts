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

vi.mock("@/lib/api-client", () => ({
  apiPost: mockApiPost,
  apiGet: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
}));

vi.mock("@/lib/api-helpers", () => ({
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)")
      ? 401
      : message.includes("(404)")
        ? 404
        : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
}));

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(
  path: string,
  options: { method?: string } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  return new NextRequest(url, { method: options.method ?? "POST" });
}

function createMockContext(
  params: Record<string, string | string[] | undefined> = {},
) {
  return { params: Promise.resolve(params) };
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("POST /api/auth/accounts/[accountId]/permissions/[permissionId]/deny", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest(
      "/api/auth/accounts/123/permissions/456/deny",
      { method: "POST" },
    );
    const response = await POST(
      request,
      createMockContext({ accountId: "123", permissionId: "456" }),
    );

    expect(response.status).toBe(401);
  });

  it("returns 400 when accountId is missing", async () => {
    const request = createMockRequest(
      "/api/auth/accounts/undefined/permissions/456/deny",
      { method: "POST" },
    );
    const response = await POST(
      request,
      createMockContext({ permissionId: "456" }),
    );

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Account ID and Permission ID are required");
  });

  it("returns 400 when permissionId is missing", async () => {
    const request = createMockRequest(
      "/api/auth/accounts/123/permissions/undefined/deny",
      { method: "POST" },
    );
    const response = await POST(
      request,
      createMockContext({ accountId: "123" }),
    );

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Account ID and Permission ID are required");
  });

  it("denies permission successfully", async () => {
    mockApiPost.mockResolvedValueOnce(undefined);

    const request = createMockRequest(
      "/api/auth/accounts/123/permissions/456/deny",
      { method: "POST" },
    );
    const response = await POST(
      request,
      createMockContext({ accountId: "123", permissionId: "456" }),
    );

    expect(mockApiPost).toHaveBeenCalledWith(
      "/auth/accounts/123/permissions/456/deny",
      {},
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{ success: boolean }>(response);
    expect(json.success).toBe(true);
  });

  it("handles API errors gracefully", async () => {
    mockApiPost.mockRejectedValueOnce(new Error("Permission not found (404)"));

    const request = createMockRequest(
      "/api/auth/accounts/123/permissions/999/deny",
      { method: "POST" },
    );
    const response = await POST(
      request,
      createMockContext({ accountId: "123", permissionId: "999" }),
    );

    expect(response.status).toBe(404);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Permission not found");
  });

  it("handles unauthorized deny attempts", async () => {
    mockApiPost.mockRejectedValueOnce(new Error("Unauthorized (401)"));

    const request = createMockRequest(
      "/api/auth/accounts/123/permissions/456/deny",
      { method: "POST" },
    );
    const response = await POST(
      request,
      createMockContext({ accountId: "123", permissionId: "456" }),
    );

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Unauthorized");
  });
});
