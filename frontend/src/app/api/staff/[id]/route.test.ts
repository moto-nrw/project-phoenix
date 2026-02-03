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

describe("GET /api/staff/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/staff/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("fetches staff member by ID from backend", async () => {
    const mockStaff = {
      status: "success",
      data: {
        id: 1,
        person_id: 100,
        staff_notes: "Test notes",
        is_teacher: true,
        teacher_id: 50,
        specialization: "Math",
        role: "Teacher",
        qualifications: "PhD",
        person: {
          id: 100,
          first_name: "John",
          last_name: "Doe",
          email: "john.doe@example.com",
          tag_id: "TAG123",
          account_id: 10,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        },
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
    };
    mockApiGet.mockResolvedValueOnce(mockStaff);

    const request = createMockRequest("/api/staff/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(mockApiGet).toHaveBeenCalledWith("/api/staff/1", "test-token");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<{ id: string; name: string }>>(
        response,
      );
    expect(json.data.id).toBe("1");
    expect(json.data.name).toBe("John Doe");
  });

  it("throws error when staff member not found", async () => {
    mockApiGet.mockResolvedValueOnce({ data: null });

    const request = createMockRequest("/api/staff/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
  });

  it("throws error when ID is missing", async () => {
    const request = createMockRequest("/api/staff/undefined");
    const response = await GET(request, createMockContext({}));

    expect(response.status).toBe(500);
  });
});

describe("PUT /api/staff/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/staff/1", {
      method: "PUT",
      body: { role: "Senior Teacher" },
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("updates staff member via backend", async () => {
    const updateBody = { role: "Senior Teacher", specialization: "Physics" };
    const mockUpdatedStaff = {
      id: 1,
      person_id: 100,
      staff_notes: null,
      is_teacher: true,
      role: "Senior Teacher",
      specialization: "Physics",
      qualifications: null,
      person: {
        id: 100,
        first_name: "John",
        last_name: "Doe",
        email: "john.doe@example.com",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-15T10:00:00Z",
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedStaff);

    const request = createMockRequest("/api/staff/1", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/staff/1",
      "test-token",
      updateBody,
    );
    expect(response.status).toBe(200);
  });

  it("throws error when permission denied", async () => {
    mockApiPut.mockRejectedValueOnce(new Error("Forbidden (403)"));

    const request = createMockRequest("/api/staff/1", {
      method: "PUT",
      body: { role: "Admin" },
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Permission denied");
  });

  it("throws error when staff member not found", async () => {
    mockApiPut.mockRejectedValueOnce(new Error("staff member not found (400)"));

    const request = createMockRequest("/api/staff/1", {
      method: "PUT",
      body: { role: "Teacher" },
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Staff member not found");
  });
});

describe("DELETE /api/staff/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/staff/1", { method: "DELETE" });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("deletes staff member via backend and returns 204", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/staff/1", { method: "DELETE" });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(mockApiDelete).toHaveBeenCalledWith("/api/staff/1", "test-token");
    expect(response.status).toBe(204);
  });

  it("throws error when permission denied", async () => {
    mockApiDelete.mockRejectedValueOnce(new Error("Forbidden (403)"));

    const request = createMockRequest("/api/staff/1", { method: "DELETE" });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Permission denied");
  });

  it("throws error when staff member not found", async () => {
    mockApiDelete.mockRejectedValueOnce(new Error("Not Found (404)"));

    const request = createMockRequest("/api/staff/1", { method: "DELETE" });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Staff member not found");
  });
});
