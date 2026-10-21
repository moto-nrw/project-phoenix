import { describe, it, expect, vi, beforeEach } from "vitest";
import { POST, DELETE } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

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
    API_URL: "http://server:8080",
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

// ============================================================================
// Test Helpers
// ============================================================================

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
  return new Response(status === 204 ? null : body, {
    status,
    headers: { "Content-Type": isText ? "text/plain" : "application/json" },
  });
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
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Unauthorized");
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
      "http://server:8080/auth/accounts/1/roles/2",
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

  it("replaces a backend database error with a safe problem envelope", async () => {
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

    expect(response.status).toBe(500);
    expect(await response.json()).toMatchObject({
      status: 500,
      detail: "Internal Server Error",
      code: "general.server",
    });
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

    expect(response.status).toBe(404);
    expect(await response.text()).toBe("Not found");
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
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Network error");
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
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Unauthorized");
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
      "http://server:8080/auth/accounts/1/roles/2",
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

  it("replaces a backend database error with a safe problem envelope", async () => {
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

    expect(response.status).toBe(500);
    expect(await response.json()).toMatchObject({
      status: 500,
      detail: "Internal Server Error",
      code: "general.server",
    });
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

    expect(response.status).toBe(404);
    expect(await response.text()).toBe("Not found");
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
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Network error");
  });
});
