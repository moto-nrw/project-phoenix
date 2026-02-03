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

describe("GET /api/active/analytics/student/[studentId]/attendance", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest(
      "/api/active/analytics/student/123/attendance",
    );
    const response = await GET(
      request,
      createMockContext({ studentId: "123" }),
    );

    expect(response.status).toBe(401);
  });

  it("returns 500 when studentId parameter is missing", async () => {
    const request = createMockRequest(
      "/api/active/analytics/student/undefined/attendance",
    );
    const response = await GET(request, createMockContext({}));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Invalid studentId parameter");
  });

  it("returns 500 when studentId parameter is not a string", async () => {
    const request = createMockRequest(
      "/api/active/analytics/student/123/attendance",
    );
    const response = await GET(
      request,
      createMockContext({ studentId: ["123", "456"] }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Invalid studentId parameter");
  });

  it("fetches attendance data for valid studentId", async () => {
    const mockAttendanceData = {
      student_id: 123,
      total_visits: 45,
      present_days: 40,
      absent_days: 5,
      attendance_rate: 0.889,
    };
    mockApiGet.mockResolvedValueOnce(mockAttendanceData);

    const request = createMockRequest(
      "/api/active/analytics/student/123/attendance",
    );
    const response = await GET(
      request,
      createMockContext({ studentId: "123" }),
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/analytics/student/123/attendance",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockAttendanceData>>(response);
    expect(json.data).toEqual(mockAttendanceData);
  });

  it("handles API errors gracefully", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Student not found (404)"));

    const request = createMockRequest(
      "/api/active/analytics/student/999/attendance",
    );
    const response = await GET(
      request,
      createMockContext({ studentId: "999" }),
    );

    expect(response.status).toBe(404);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Student not found");
  });
});
