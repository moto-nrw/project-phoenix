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

describe("GET /api/schedules/timeframes", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/schedules/timeframes");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches and maps timeframes from backend", async () => {
    const backendTimeframes = [
      {
        id: 1,
        start_time: "2024-01-15T08:00:00Z",
        end_time: "2024-01-15T12:00:00Z",
        is_active: true,
        description: "Morning shift",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
      {
        id: 2,
        start_time: "2024-01-15T13:00:00Z",
        end_time: "2024-01-15T17:00:00Z",
        is_active: true,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
    ];

    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: backendTimeframes,
    });

    const request = createMockRequest("/api/schedules/timeframes");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/schedules/timeframes",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]).toMatchObject({
      id: "1",
      start_time: "2024-01-15T08:00:00Z",
      end_time: "2024-01-15T12:00:00Z",
      is_active: true,
      description: "Morning shift",
      display_name: "Morning shift",
    });
  });

  it("filters out inactive timeframes", async () => {
    const backendTimeframes = [
      {
        id: 1,
        start_time: "2024-01-15T08:00:00Z",
        is_active: true,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
      {
        id: 2,
        start_time: "2024-01-15T13:00:00Z",
        is_active: false,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
    ];

    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: backendTimeframes,
    });

    const request = createMockRequest("/api/schedules/timeframes");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toHaveLength(1);
    expect(json.data[0]).toMatchObject({
      id: "1",
      is_active: true,
    });
  });

  it("generates display_name from time range when description is missing", async () => {
    const backendTimeframes = [
      {
        id: 1,
        start_time: "2024-01-15T08:00:00Z",
        end_time: "2024-01-15T12:00:00Z",
        is_active: true,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
    ];

    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: backendTimeframes,
    });

    const request = createMockRequest("/api/schedules/timeframes");
    const response = await GET(request, createMockContext());

    const json =
      await parseJsonResponse<ApiResponse<Array<Record<string, unknown>>>>(
        response,
      );
    expect(json.data[0]).toHaveProperty("display_name");
    // Display name should be formatted time range
    expect(typeof json.data[0]!.display_name).toBe("string");
  });

  it("handles response without status field", async () => {
    const backendTimeframes = [
      {
        id: 1,
        start_time: "2024-01-15T08:00:00Z",
        is_active: true,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
    ];

    mockApiGet.mockResolvedValueOnce({
      data: backendTimeframes,
    });

    const request = createMockRequest("/api/schedules/timeframes");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toHaveLength(1);
  });

  it("returns empty array on unexpected response format", async () => {
    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: null,
    });

    const request = createMockRequest("/api/schedules/timeframes");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });

  it("returns empty array when API call fails", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Backend error"));

    const request = createMockRequest("/api/schedules/timeframes");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });
});
