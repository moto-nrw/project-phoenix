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

describe("GET /api/rooms/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/rooms/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("fetches room by ID from backend (wrapped response)", async () => {
    const mockRoom = {
      id: 123,
      name: "Room A",
      building: "Main",
      floor: 2,
      capacity: 30,
      is_occupied: false,
    };
    mockApiGet.mockResolvedValueOnce({ data: mockRoom });

    const request = createMockRequest("/api/rooms/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(mockApiGet).toHaveBeenCalledWith("/api/rooms/123", "test-token");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockRoom>>(response);
    expect(json.data.id).toBe(123);
    expect(json.data.name).toBe("Room A");
  });

  it("fetches room by ID from backend (direct response)", async () => {
    const mockRoom = {
      id: 456,
      name: "Room B",
      is_occupied: true,
      student_count: 15,
    };
    mockApiGet.mockResolvedValueOnce(mockRoom);

    const request = createMockRequest("/api/rooms/456");
    const response = await GET(request, createMockContext({ id: "456" }));

    const json =
      await parseJsonResponse<ApiResponse<typeof mockRoom>>(response);
    expect(json.data.id).toBe(456);
    expect(json.data.student_count).toBe(15);
  });

  it("handles 404 room not found", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("API error (404): Not found"));

    const request = createMockRequest("/api/rooms/999");
    const response = await GET(request, createMockContext({ id: "999" }));

    expect(response.status).toBe(404);
  });
});

describe("PUT /api/rooms/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/rooms/123", {
      method: "PUT",
      body: { name: "Updated Room" },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("updates room via backend", async () => {
    const updateBody = { name: "Updated Room", capacity: 40 };
    const mockUpdatedRoom = {
      id: 123,
      name: "Updated Room",
      capacity: 40,
      is_occupied: false,
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedRoom);

    const request = createMockRequest("/api/rooms/123", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/rooms/123",
      "test-token",
      expect.objectContaining({
        name: "Updated Room",
        capacity: 40,
      }),
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedRoom>>(response);
    expect(json.data.name).toBe("Updated Room");
  });

  it("rejects update with invalid capacity", async () => {
    const request = createMockRequest("/api/rooms/123", {
      method: "PUT",
      body: { capacity: -5 },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Capacity must be greater than 0");
  });

  it("handles camelCase to snake_case field mapping", async () => {
    const updateBody = { deviceId: "device-123" };
    const mockUpdatedRoom = { id: 123, name: "Room", device_id: "device-123" };
    mockApiPut.mockResolvedValueOnce(mockUpdatedRoom);

    const request = createMockRequest("/api/rooms/123", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/rooms/123",
      "test-token",
      expect.objectContaining({
        device_id: "device-123",
      }),
    );
    expect(response.status).toBe(200);
  });
});

describe("DELETE /api/rooms/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/rooms/123", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("deletes room via backend and returns 204", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/rooms/123", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(mockApiDelete).toHaveBeenCalledWith("/api/rooms/123", "test-token");
    expect(response.status).toBe(204);
  });

  it("handles 404 room not found", async () => {
    mockApiDelete.mockRejectedValueOnce(
      new Error("API error (404): Room not found"),
    );

    const request = createMockRequest("/api/rooms/999", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "999" }));

    expect(response.status).toBe(500);
  });
});
