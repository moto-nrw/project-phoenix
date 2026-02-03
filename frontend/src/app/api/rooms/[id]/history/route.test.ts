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

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
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

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

interface ApiResponse<T> {
  status: string;
  data: T;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/rooms/[id]/history", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/rooms/123/history");
    const response = await GET(request);

    expect(response.status).toBe(401);
    expect(mockApiGet).not.toHaveBeenCalled();
  });

  it("fetches room history from backend", async () => {
    const mockHistory = [
      {
        id: 1,
        room_id: 123,
        date: "2024-01-15",
        group_name: "OGS A",
        student_count: 15,
        duration: 120,
      },
      {
        id: 2,
        room_id: 123,
        date: "2024-01-14",
        group_name: "OGS B",
        activity_name: "Art Class",
        supervisor_name: "Ms. Smith",
        student_count: 12,
        duration: 90,
      },
    ];

    mockApiGet.mockResolvedValueOnce(mockHistory);

    const request = createMockRequest("/api/rooms/123/history");
    const response = await GET(request);

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/rooms/123/history",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockHistory>>(response);
    expect(json.status).toBe("success");
    expect(json.data).toEqual(mockHistory);
  });

  it("supports date range query parameters", async () => {
    mockApiGet.mockResolvedValueOnce([]);

    const request = createMockRequest(
      "/api/rooms/123/history?start_date=2024-01-01&end_date=2024-01-31",
    );
    await GET(request);

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/rooms/123/history?start_date=2024-01-01&end_date=2024-01-31",
      "test-token",
    );
  });

  it("supports partial date parameters", async () => {
    mockApiGet.mockResolvedValueOnce([]);

    const request = createMockRequest(
      "/api/rooms/123/history?start_date=2024-01-01",
    );
    await GET(request);

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/rooms/123/history?start_date=2024-01-01",
      "test-token",
    );
  });

  it("returns empty array when room has no history (404)", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Not found (404)"));

    const request = createMockRequest("/api/rooms/999/history");
    const response = await GET(request);

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.status).toBe("success");
    expect(json.data).toEqual([]);
  });

  it("returns 400 when room ID is missing", async () => {
    const request = createMockRequest("/api/rooms//history");
    const response = await GET(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Invalid id parameter");
    expect(mockApiGet).not.toHaveBeenCalled();
  });

  it("handles backend errors", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Database connection failed"));

    const request = createMockRequest("/api/rooms/123/history");
    const response = await GET(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Backend API error");
  });

  it("handles non-Error rejections", async () => {
    mockApiGet.mockRejectedValueOnce("String error");

    const request = createMockRequest("/api/rooms/123/history");
    const response = await GET(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("String error");
  });

  it("extracts room ID from URL path correctly", async () => {
    mockApiGet.mockResolvedValueOnce([]);

    const request = createMockRequest("/api/rooms/456/history");
    await GET(request);

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/rooms/456/history",
      "test-token",
    );
  });
});
