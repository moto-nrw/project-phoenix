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

describe("GET /api/active/combined/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/combined/5");
    const response = await GET(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(401);
  });

  it("fetches combined group by ID from backend", async () => {
    const mockCombinedGroup = {
      id: 5,
      name: "Combined Group A",
      description: "Test group",
      room_id: 10,
    };
    mockApiGet.mockResolvedValueOnce(mockCombinedGroup);

    const request = createMockRequest("/api/active/combined/5");
    const response = await GET(request, createMockContext({ id: "5" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/combined/5",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockCombinedGroup>>(response);
    expect(json.data).toEqual(mockCombinedGroup);
  });

  it("handles non-existent combined group", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Not found (404)"));

    const request = createMockRequest("/api/active/combined/999");
    const response = await GET(request, createMockContext({ id: "999" }));

    expect(response.status).toBe(404);
  });
});

describe("PUT /api/active/combined/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/combined/5", {
      method: "PUT",
      body: { name: "Updated Name" },
    });
    const response = await PUT(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(401);
  });

  it("updates combined group via backend", async () => {
    const updateBody = {
      name: "Updated Combined Group",
      description: "Updated description",
    };
    const mockUpdatedGroup = {
      id: 5,
      name: "Updated Combined Group",
      description: "Updated description",
      room_id: 10,
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedGroup);

    const request = createMockRequest("/api/active/combined/5", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "5" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/active/combined/5",
      "test-token",
      updateBody,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedGroup>>(response);
    expect(json.data.name).toBe("Updated Combined Group");
  });

  it("updates only specific fields", async () => {
    const updateBody = { room_id: "20" };
    const mockUpdatedGroup = {
      id: 5,
      name: "Original Name",
      description: "Original description",
      room_id: 20,
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedGroup);

    const request = createMockRequest("/api/active/combined/5", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedGroup>>(response);
    expect(json.data.room_id).toBe(20);
  });

  it("handles backend errors gracefully", async () => {
    mockApiPut.mockRejectedValueOnce(new Error("Backend error (500)"));

    const request = createMockRequest("/api/active/combined/5", {
      method: "PUT",
      body: { name: "Test" },
    });
    const response = await PUT(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
  });
});

describe("DELETE /api/active/combined/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/combined/5", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(401);
  });

  it("deletes combined group via backend and returns 204", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/active/combined/5", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "5" }));

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/active/combined/5",
      "test-token",
    );
    expect(response.status).toBe(204);
  });

  it("handles non-existent combined group", async () => {
    mockApiDelete.mockRejectedValueOnce(new Error("Not found (404)"));

    const request = createMockRequest("/api/active/combined/999", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "999" }));

    expect(response.status).toBe(404);
  });

  it("handles backend errors gracefully", async () => {
    mockApiDelete.mockRejectedValueOnce(new Error("Backend error (500)"));

    const request = createMockRequest("/api/active/combined/5", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
  });
});
