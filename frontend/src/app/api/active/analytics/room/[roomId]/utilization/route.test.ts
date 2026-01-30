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

describe("GET /api/active/analytics/room/[roomId]/utilization", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest(
      "/api/active/analytics/room/123/utilization",
    );
    const response = await GET(request, createMockContext({ roomId: "123" }));

    expect(response.status).toBe(401);
  });

  it("fetches room utilization from backend", async () => {
    const mockUtilization = {
      room_id: 123,
      total_hours: 40,
      occupied_hours: 28,
      utilization_percentage: 70,
      sessions: [
        {
          date: "2024-01-15",
          group_name: "OGS A",
          duration_minutes: 120,
        },
      ],
    };

    mockApiGet.mockResolvedValueOnce(mockUtilization);

    const request = createMockRequest(
      "/api/active/analytics/room/123/utilization",
    );
    const response = await GET(request, createMockContext({ roomId: "123" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/analytics/room/123/utilization",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUtilization>>(response);
    expect(json.data).toEqual(mockUtilization);
  });

  it("handles numeric roomId parameter", async () => {
    const mockUtilization = {
      room_id: 456,
      utilization_percentage: 85,
    };

    mockApiGet.mockResolvedValueOnce(mockUtilization);

    const request = createMockRequest(
      "/api/active/analytics/room/456/utilization",
    );
    const response = await GET(request, createMockContext({ roomId: "456" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/analytics/room/456/utilization",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("returns 500 when roomId is missing", async () => {
    const request = createMockRequest("/api/active/analytics/room/utilization");
    const response = await GET(request, createMockContext({}));

    expect(response.status).toBe(500);
    expect(mockApiGet).not.toHaveBeenCalled();
  });

  it("returns 500 when roomId is invalid type", async () => {
    const request = createMockRequest(
      "/api/active/analytics/room/invalid/utilization",
    );
    const response = await GET(
      request,
      createMockContext({ roomId: { invalid: true } } as unknown as Record<
        string,
        string
      >),
    );

    expect(response.status).toBe(500);
    expect(mockApiGet).not.toHaveBeenCalled();
  });

  it("handles room not found error (404)", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Room not found (404)"));

    const request = createMockRequest(
      "/api/active/analytics/room/999/utilization",
    );
    const response = await GET(request, createMockContext({ roomId: "999" }));

    expect(response.status).toBe(404);
  });

  it("forwards backend server errors", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Database error (500)"));

    const request = createMockRequest(
      "/api/active/analytics/room/123/utilization",
    );
    const response = await GET(request, createMockContext({ roomId: "123" }));

    expect(response.status).toBe(500);
  });

  it("handles undefined roomId parameter", async () => {
    const request = createMockRequest(
      "/api/active/analytics/room/123/utilization",
    );
    const response = await GET(
      request,
      createMockContext({ roomId: undefined }),
    );

    expect(response.status).toBe(500);
    expect(mockApiGet).not.toHaveBeenCalled();
  });
});
