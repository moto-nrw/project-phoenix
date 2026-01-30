import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { PUT } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockApiPut } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPut: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("@/lib/api-client", () => ({
  apiPut: mockApiPut,
  apiGet: vi.fn(),
  apiPost: vi.fn(),
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
  options: { method?: string; body?: unknown } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const requestInit: { method: string; body?: string; headers?: HeadersInit } =
    {
      method: options.method ?? "PUT",
    };

  if (options.body) {
    requestInit.body = JSON.stringify(options.body);
    requestInit.headers = { "Content-Type": "application/json" };
  }

  return new NextRequest(url, requestInit);
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

describe("PUT /api/auth/accounts/[accountId]/deactivate", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/accounts/123/deactivate", {
      method: "PUT",
    });
    const response = await PUT(
      request,
      createMockContext({ accountId: "123" }),
    );

    expect(response.status).toBe(401);
  });

  it("deactivates account successfully", async () => {
    const mockResponse = { data: { message: "Account deactivated" } };
    mockApiPut.mockResolvedValueOnce(mockResponse);

    const request = createMockRequest("/api/auth/accounts/123/deactivate", {
      method: "PUT",
    });
    const response = await PUT(
      request,
      createMockContext({ accountId: "123" }),
    );

    expect(mockApiPut).toHaveBeenCalledWith(
      "/auth/accounts/123/deactivate",
      null,
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<{ message: string }>>(response);
    expect(json.data.message).toBe("Account deactivated");
  });

  it("handles API errors gracefully", async () => {
    mockApiPut.mockRejectedValueOnce(new Error("Account not found (404)"));

    const request = createMockRequest("/api/auth/accounts/999/deactivate", {
      method: "PUT",
    });
    const response = await PUT(
      request,
      createMockContext({ accountId: "999" }),
    );

    expect(response.status).toBe(404);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Account not found");
  });

  it("handles unauthorized deactivation attempts", async () => {
    mockApiPut.mockRejectedValueOnce(new Error("Unauthorized (401)"));

    const request = createMockRequest("/api/auth/accounts/456/deactivate", {
      method: "PUT",
    });
    const response = await PUT(
      request,
      createMockContext({ accountId: "456" }),
    );

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Unauthorized");
  });
});
