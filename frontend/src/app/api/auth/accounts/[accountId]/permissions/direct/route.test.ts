import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("@/lib/api-client", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
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

function createMockRequest(path: string): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  return new NextRequest(url);
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

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/auth/accounts/[accountId]/permissions/direct", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest(
      "/api/auth/accounts/123/permissions/direct",
    );
    const response = await GET(
      request,
      createMockContext({ accountId: "123" }),
    );

    expect(response.status).toBe(401);
  });

  it("returns 500 when accountId parameter is missing", async () => {
    const request = createMockRequest(
      "/api/auth/accounts/undefined/permissions/direct",
    );
    const response = await GET(request, createMockContext({}));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Account ID is required");
  });

  it("fetches direct permissions for valid accountId", async () => {
    const mockPermissions = [
      {
        id: 3,
        name: "activities:admin",
        description: "Admin activities",
        resource: "activities",
        action: "admin",
      },
    ];
    const mockResponse = {
      status: "success",
      data: mockPermissions,
    };
    mockApiGet.mockResolvedValueOnce(mockResponse);

    const request = createMockRequest(
      "/api/auth/accounts/123/permissions/direct",
    );
    const response = await GET(
      request,
      createMockContext({ accountId: "123" }),
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/auth/accounts/123/permissions/direct",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockPermissions>>(response);
    expect(json.data).toEqual(mockPermissions);
  });

  it("returns empty array when account has no direct permissions", async () => {
    const mockResponse = {
      status: "success",
      data: [],
    };
    mockApiGet.mockResolvedValueOnce(mockResponse);

    const request = createMockRequest(
      "/api/auth/accounts/123/permissions/direct",
    );
    const response = await GET(
      request,
      createMockContext({ accountId: "123" }),
    );

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });

  it("handles API errors gracefully", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Account not found (404)"));

    const request = createMockRequest(
      "/api/auth/accounts/999/permissions/direct",
    );
    const response = await GET(
      request,
      createMockContext({ accountId: "999" }),
    );

    expect(response.status).toBe(404);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Account not found");
  });
});
