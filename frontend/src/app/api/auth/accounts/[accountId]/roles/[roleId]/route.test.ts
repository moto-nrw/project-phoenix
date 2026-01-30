import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { POST, DELETE } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

global.fetch = mockFetch as typeof fetch;

// Mock env
vi.mock("~/env", () => ({
  env: {
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
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
      method: options.method ?? "GET",
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

function createMockFetchResponse(
  status: number,
  data?: unknown,
  isText = false,
): Response {
  const body = isText ? (data as string) : JSON.stringify(data);
  return {
    status,
    ok: status >= 200 && status < 300,
    json: async () => data,
    text: async () => body,
  } as Response;
}

// ============================================================================
// Tests
// ============================================================================

describe("POST /api/auth/accounts/[accountId]/roles/[roleId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/accounts/1/roles/2", {
      method: "POST",
    });
    const response = await POST(
      request,
      createMockContext({ accountId: "1", roleId: "2" }),
    );

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Unauthorized");
  });

  it("assigns role and returns success on 204 response", async () => {
    mockFetch.mockResolvedValueOnce(createMockFetchResponse(204));

    const request = createMockRequest("/api/auth/accounts/1/roles/2", {
      method: "POST",
    });
    const response = await POST(
      request,
      createMockContext({ accountId: "1", roleId: "2" }),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/accounts/1/roles/2",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<ApiResponse<null>>(response);
    expect(json.success).toBe(true);
    expect(json.message).toBe("Role assigned successfully");
    expect(json.data).toBeNull();
  });

  it("returns JSON data when backend returns non-204 success", async () => {
    const mockData = { account_id: 1, role_id: 2 };
    mockFetch.mockResolvedValueOnce(createMockFetchResponse(200, mockData));

    const request = createMockRequest("/api/auth/accounts/1/roles/2", {
      method: "POST",
    });
    const response = await POST(
      request,
      createMockContext({ accountId: "1", roleId: "2" }),
    );

    expect(response.status).toBe(200);
    const json =
      await parseJsonResponse<ApiResponse<typeof mockData>>(response);
    expect(json.success).toBe(true);
    expect(json.data).toEqual(mockData);
  });

  it("handles database schema mismatch error", async () => {
    const errorText =
      "SQL error: missing FROM-clause entry for table account_role";
    mockFetch.mockResolvedValueOnce(
      createMockFetchResponse(500, errorText, true),
    );

    const request = createMockRequest("/api/auth/accounts/1/roles/2", {
      method: "POST",
    });
    const response = await POST(
      request,
      createMockContext({ accountId: "1", roleId: "2" }),
    );

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<{
      success: boolean;
      message: string;
      error: string;
    }>(response);
    expect(json.success).toBe(false);
    expect(json.message).toContain("Database schema mismatch");
    expect(json.error).toContain("Backend database configuration error");
  });

  it("returns 500 on API error", async () => {
    mockFetch.mockResolvedValueOnce(
      createMockFetchResponse(404, "Not found", true),
    );

    const request = createMockRequest("/api/auth/accounts/1/roles/2", {
      method: "POST",
    });
    const response = await POST(
      request,
      createMockContext({ accountId: "1", roleId: "2" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("API error (404)");
  });

  it("handles fetch exceptions", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/auth/accounts/1/roles/2", {
      method: "POST",
    });
    const response = await POST(
      request,
      createMockContext({ accountId: "1", roleId: "2" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Network error");
  });
});

describe("DELETE /api/auth/accounts/[accountId]/roles/[roleId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/accounts/1/roles/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ accountId: "1", roleId: "2" }),
    );

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Unauthorized");
  });

  it("removes role and returns success on 204 response", async () => {
    mockFetch.mockResolvedValueOnce(createMockFetchResponse(204));

    const request = createMockRequest("/api/auth/accounts/1/roles/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ accountId: "1", roleId: "2" }),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/accounts/1/roles/2",
      expect.objectContaining({
        method: "DELETE",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<ApiResponse<null>>(response);
    expect(json.success).toBe(true);
    expect(json.message).toBe("Role removed successfully");
    expect(json.data).toBeNull();
  });

  it("returns JSON data when backend returns non-204 success", async () => {
    const mockData = { account_id: 1, role_id: 2 };
    mockFetch.mockResolvedValueOnce(createMockFetchResponse(200, mockData));

    const request = createMockRequest("/api/auth/accounts/1/roles/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ accountId: "1", roleId: "2" }),
    );

    expect(response.status).toBe(200);
    const json =
      await parseJsonResponse<ApiResponse<typeof mockData>>(response);
    expect(json.success).toBe(true);
    expect(json.data).toEqual(mockData);
  });

  it("handles database schema mismatch error", async () => {
    const errorText =
      "SQL error: missing FROM-clause entry for table account_role";
    mockFetch.mockResolvedValueOnce(
      createMockFetchResponse(500, errorText, true),
    );

    const request = createMockRequest("/api/auth/accounts/1/roles/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ accountId: "1", roleId: "2" }),
    );

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<{
      success: boolean;
      message: string;
      error: string;
    }>(response);
    expect(json.success).toBe(false);
    expect(json.message).toContain("Database schema mismatch");
    expect(json.error).toContain("Backend database configuration error");
  });

  it("returns 500 on API error", async () => {
    mockFetch.mockResolvedValueOnce(
      createMockFetchResponse(404, "Not found", true),
    );

    const request = createMockRequest("/api/auth/accounts/1/roles/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ accountId: "1", roleId: "2" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("API error (404)");
  });

  it("handles fetch exceptions", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/auth/accounts/1/roles/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ accountId: "1", roleId: "2" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Network error");
  });
});
