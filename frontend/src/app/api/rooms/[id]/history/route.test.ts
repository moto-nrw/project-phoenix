import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET } from "./route";
import { ApiResponseError } from "~/lib/api-helpers";

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

// Re-export the real ApiResponseError so route's `instanceof` check matches
// the same class the test throws. Keep apiGet etc. mocked so the route
// never hits the network.
vi.mock("~/lib/api-helpers", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/api-helpers")>();
  return {
    ApiResponseError: actual.ApiResponseError,
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
  };
});

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

  it("unwraps the backend envelope and passes the session array to the client", async () => {
    // Issue #1425: backend now returns one aggregated session per row —
    // no per-student fields. Mirrors RoomSessionEntry on the Go side.
    // common.Respond wraps the array in { status, data, message }; the
    // route unwraps that envelope so the drawer can consume `.data` as an
    // array (it used to be nested two levels deep and rendered empty).
    const mockHistory = [
      {
        session_id: 1,
        started_at: "2024-01-15T08:00:00Z",
        ended_at: "2024-01-15T10:00:00Z",
        duration_minutes: 120,
        activity_name: "OGS A",
        supervisor_name: "Ms. Smith",
        student_count: 15,
      },
      {
        session_id: 2,
        started_at: "2024-01-14T13:00:00Z",
        ended_at: "2024-01-14T14:30:00Z",
        duration_minutes: 90,
        activity_name: "Art Class",
        supervisor_name: "Mr. Jones",
        student_count: 12,
      },
    ];

    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: mockHistory,
      message: "Room history retrieved successfully",
    });

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

  it("returns an empty array when the backend envelope carries data:null", async () => {
    // common.Respond can emit { status: "success", data: null } when the
    // service returns nil. Don't let that leak through as `null` to the
    // drawer (which would crash on .map).
    mockApiGet.mockResolvedValueOnce({ status: "success", data: null });

    const request = createMockRequest("/api/rooms/123/history");
    const response = await GET(request);

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });

  it("forwards RFC3339 start/end query params", async () => {
    mockApiGet.mockResolvedValueOnce({ status: "success", data: [] });

    const request = createMockRequest(
      "/api/rooms/123/history?start=2024-01-01T00:00:00Z&end=2024-01-31T23:59:59Z",
    );
    await GET(request);

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/rooms/123/history?start=2024-01-01T00%3A00%3A00Z&end=2024-01-31T23%3A59%3A59Z",
      "test-token",
    );
  });

  it("translates legacy start_date/end_date params into backend-canonical start/end", async () => {
    // Issue #1425 fix: backend expects `start`/`end`, the old route forwarded
    // `start_date`/`end_date` verbatim — silently broken. The proxy now
    // rewrites the legacy names so existing callers keep working.
    mockApiGet.mockResolvedValueOnce({ status: "success", data: [] });

    const request = createMockRequest(
      "/api/rooms/123/history?start_date=2024-01-01T00:00:00Z&end_date=2024-01-31T23:59:59Z",
    );
    await GET(request);

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/rooms/123/history?start=2024-01-01T00%3A00%3A00Z&end=2024-01-31T23%3A59%3A59Z",
      "test-token",
    );
  });

  it("normalizes legacy date-only range params to RFC3339 bounds", async () => {
    mockApiGet.mockResolvedValueOnce({ status: "success", data: [] });

    const request = createMockRequest(
      "/api/rooms/123/history?start_date=2024-01-01&end_date=2024-01-31",
    );
    await GET(request);

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/rooms/123/history?start=2024-01-01T00%3A00%3A00Z&end=2024-01-31T23%3A59%3A59Z",
      "test-token",
    );
  });

  it("supports partial date parameters", async () => {
    mockApiGet.mockResolvedValueOnce({ status: "success", data: [] });

    const request = createMockRequest(
      "/api/rooms/123/history?start=2024-01-01T00:00:00Z",
    );
    await GET(request);

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/rooms/123/history?start=2024-01-01T00%3A00%3A00Z",
      "test-token",
    );
  });

  it("returns empty array when room has no history (404)", async () => {
    mockApiGet.mockRejectedValueOnce(
      new ApiResponseError(404, '{"status":"error","error":"not found"}'),
    );

    const request = createMockRequest("/api/rooms/999/history");
    const response = await GET(request);

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.status).toBe("success");
    expect(json.data).toEqual([]);
  });

  it("translates a 403 feature_disabled error into a 200 + status:feature_disabled signal", async () => {
    // Issue #1425 follow-up: when gdpr.attendance_log_enabled is off the
    // backend returns 403 with body { status: "error", error:
    // "feature_disabled" }. The proxy must NOT treat that as a server
    // error — instead it returns a structured "the feature is off" signal
    // (HTTP 200) so the drawer can hide the section deliberately. A plain
    // 403 without the feature_disabled marker still falls through to the
    // generic error path (covered by "handles backend errors" below).
    mockApiGet.mockRejectedValueOnce(
      new ApiResponseError(
        403,
        '{"status":"error","error":"feature_disabled"}',
      ),
    );

    const request = createMockRequest("/api/rooms/123/history");
    const response = await GET(request);

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<{
      status: string;
      data: unknown[];
    }>(response);
    expect(json.status).toBe("feature_disabled");
    expect(json.data).toEqual([]);
  });

  it("does NOT translate a generic 403 (no feature_disabled marker) into the disabled signal", async () => {
    // A plain RBAC failure must still surface as a 500 so it gets logged.
    // Otherwise we'd silently mask permission bugs as "feature off".
    mockApiGet.mockRejectedValueOnce(
      new ApiResponseError(403, '{"status":"error","error":"forbidden"}'),
    );

    const request = createMockRequest("/api/rooms/123/history");
    const response = await GET(request);

    expect(response.status).toBe(500);
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
    mockApiGet.mockResolvedValueOnce({ status: "success", data: [] });

    const request = createMockRequest("/api/rooms/456/history");
    await GET(request);

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/rooms/456/history",
      "test-token",
    );
  });
});
