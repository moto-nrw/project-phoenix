import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, PUT, DELETE } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiGet, mockApiPut, mockApiDelete } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPut: vi.fn(),
  mockApiDelete: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: mockApiPut,
  apiDelete: mockApiDelete,
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)")
      ? 401
      : message.includes("(404)")
        ? 404
        : message.includes("(403)")
          ? 403
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

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/users/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/users/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("fetches user by ID from backend", async () => {
    const mockUser = {
      status: "success",
      data: {
        id: 1,
        first_name: "John",
        last_name: "Doe",
        tag_id: "TAG123",
        account_id: 10,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
    };
    mockApiGet.mockResolvedValueOnce(mockUser);

    const request = createMockRequest("/api/users/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(mockApiGet).toHaveBeenCalledWith("/api/users/1", "test-token");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<{ id: number; first_name: string }>>(
        response,
      );
    expect(json.data.id).toBe(1);
    expect(json.data.first_name).toBe("John");
  });

  it("throws error when user not found", async () => {
    mockApiGet.mockResolvedValueOnce({ data: null });

    const request = createMockRequest("/api/users/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Person not found");
  });

  it("throws error when ID is missing", async () => {
    const request = createMockRequest("/api/users/undefined");
    const response = await GET(request, createMockContext({}));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Person ID is required");
  });
});

describe("PUT /api/users/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/users/1", {
      method: "PUT",
      body: { first_name: "Jane" },
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("updates user via backend", async () => {
    const updateBody = { first_name: "Jane", last_name: "Doe" };
    const mockUpdatedUser = {
      id: 1,
      first_name: "Jane",
      last_name: "Doe",
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-15T10:00:00Z",
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedUser);

    const request = createMockRequest("/api/users/1", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/users/1",
      "test-token",
      updateBody,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<{ first_name: string }>>(response);
    expect(json.data.first_name).toBe("Jane");
  });

  it("throws error when first_name is blank", async () => {
    const request = createMockRequest("/api/users/1", {
      method: "PUT",
      body: { first_name: "  " },
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("First name cannot be blank");
  });

  it("throws error when last_name is blank", async () => {
    const request = createMockRequest("/api/users/1", {
      method: "PUT",
      body: { last_name: "" },
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Last name cannot be blank");
  });

  it("throws error when permission denied", async () => {
    mockApiPut.mockRejectedValueOnce(new Error("Forbidden (403)"));

    const request = createMockRequest("/api/users/1", {
      method: "PUT",
      body: { first_name: "Jane" },
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Permission denied");
  });

  it("throws error when user not found", async () => {
    mockApiPut.mockRejectedValueOnce(new Error("person not found (400)"));

    const request = createMockRequest("/api/users/1", {
      method: "PUT",
      body: { first_name: "Jane" },
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Person not found");
  });
});

describe("DELETE /api/users/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/users/1", { method: "DELETE" });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("deletes user via backend and returns 204", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/users/1", { method: "DELETE" });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(mockApiDelete).toHaveBeenCalledWith("/api/users/1", "test-token");
    expect(response.status).toBe(204);
  });

  it("throws error when permission denied", async () => {
    mockApiDelete.mockRejectedValueOnce(new Error("Forbidden (403)"));

    const request = createMockRequest("/api/users/1", { method: "DELETE" });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Permission denied");
  });

  it("throws error when user not found", async () => {
    mockApiDelete.mockRejectedValueOnce(new Error("Not Found (404)"));

    const request = createMockRequest("/api/users/1", { method: "DELETE" });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Person not found");
  });
});
