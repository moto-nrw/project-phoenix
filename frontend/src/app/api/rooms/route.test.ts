import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, POST } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiGet, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: mockApiPost,
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
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

describe("GET /api/rooms", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/rooms");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches rooms from backend API", async () => {
    const mockRooms = [
      { id: 1, name: "Room A", is_occupied: false },
      { id: 2, name: "Room B", is_occupied: true },
    ];
    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: mockRooms,
      pagination: {
        current_page: 1,
        page_size: 50,
        total_pages: 1,
        total_records: 2,
      },
    });

    const request = createMockRequest("/api/rooms");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith("/api/rooms", "test-token");
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      data: typeof mockRooms;
      pagination: {
        current_page: number;
        page_size: number;
        total_pages: number;
        total_records: number;
      };
      status: string;
    }>(response);
    expect(json.data).toEqual(mockRooms);
  });

  it("forwards query parameters to backend", async () => {
    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: [],
      pagination: {
        current_page: 1,
        page_size: 50,
        total_pages: 0,
        total_records: 0,
      },
    });

    const request = createMockRequest("/api/rooms?category=classroom&floor=2");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/rooms?category=classroom&floor=2",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("returns empty data when API returns null", async () => {
    mockApiGet.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/rooms");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<unknown[]>(response);
    expect(json).toEqual([]);
  });

  it("handles errors gracefully", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/rooms");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<{
      data: unknown[];
      pagination: {
        current_page: number;
        page_size: number;
        total_pages: number;
        total_records: number;
      };
    }>(response);
    expect(json.data).toEqual([]);
  });
});

describe("POST /api/rooms", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/rooms", {
      method: "POST",
      body: { name: "New Room" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("creates a new room via backend API", async () => {
    const createRequest = {
      name: "New Room",
      building: "Building A",
      floor: 2,
      capacity: 30,
    };
    const mockCreatedRoom = {
      id: 1,
      name: "New Room",
      building: "Building A",
      floor: 2,
      capacity: 30,
      is_occupied: false,
    };
    mockApiPost.mockResolvedValueOnce(mockCreatedRoom);

    const request = createMockRequest("/api/rooms", {
      method: "POST",
      body: createRequest,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/rooms",
      "test-token",
      createRequest,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockCreatedRoom>>(response);
    expect(json.data).toEqual(mockCreatedRoom);
  });

  it("rejects room creation with empty name", async () => {
    const request = createMockRequest("/api/rooms", {
      method: "POST",
      body: { name: "" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("name cannot be blank");
  });

  it("rejects room creation with invalid capacity", async () => {
    const request = createMockRequest("/api/rooms", {
      method: "POST",
      body: { name: "Room", capacity: 0 },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Capacity must be greater than 0");
  });

  it("handles permission denied error", async () => {
    mockApiPost.mockRejectedValueOnce(new Error("API error (403): Forbidden"));

    const request = createMockRequest("/api/rooms", {
      method: "POST",
      body: { name: "Room" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
  });
});
