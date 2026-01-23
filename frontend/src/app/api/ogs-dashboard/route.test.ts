import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { GET } from "./route";

// ============================================================================
// Mocks
// ============================================================================

const { mockApiGet } = vi.hoisted(() => ({
  mockApiGet: vi.fn(),
}));

// Note: auth() is globally mocked in setup.ts
// It checks for better-auth.session_token cookie

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

// BetterAuth: Expected cookie header (from global setup.ts mock)
const TEST_COOKIE_HEADER = "better-auth.session_token=test-session-token";

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

describe("GET /api/ogs-dashboard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Global mock provides authenticated session by default
  });

  it("returns 401 when not authenticated", async () => {
    // Mock cookies to return undefined (no session cookie)
    const { cookies } = await import("next/headers");
    vi.mocked(cookies).mockResolvedValueOnce({
      get: vi.fn(() => undefined),
      toString: vi.fn(() => ""),
    } as never);

    const request = createMockRequest("/api/ogs-dashboard");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("returns empty payload when user has no groups", async () => {
    mockApiGet.mockResolvedValueOnce({ data: [] });

    const request = createMockRequest("/api/ogs-dashboard");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledTimes(1);
    // BetterAuth: Now uses cookie header instead of token
    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/me/groups",
      TEST_COOKIE_HEADER,
    );

    const json = await parseJsonResponse<
      ApiResponse<{
        groups: unknown[];
        students: unknown[];
        roomStatus: unknown;
        substitutions: unknown[];
        firstGroupId: string | null;
      }>
    >(response);

    expect(json.data.groups).toEqual([]);
    expect(json.data.students).toEqual([]);
    expect(json.data.roomStatus).toBeNull();
    expect(json.data.substitutions).toEqual([]);
    expect(json.data.firstGroupId).toBeNull();
  });

  it("fetches dashboard data for the first group", async () => {
    const groups = [{ id: 12, name: "OGS A" }];
    const students = [{ id: 1, first_name: "Mia", second_name: "M" }];
    const roomStatus = { group_has_room: true, student_room_status: {} };
    const substitutions = [{ id: 99, group_id: 12 }];

    mockApiGet
      .mockResolvedValueOnce({ data: groups })
      .mockResolvedValueOnce({ data: students })
      .mockResolvedValueOnce({ data: roomStatus })
      .mockResolvedValueOnce({ data: substitutions });

    const request = createMockRequest("/api/ogs-dashboard");
    const response = await GET(request, createMockContext());

    // BetterAuth: Now uses cookie header instead of token
    expect(mockApiGet).toHaveBeenNthCalledWith(
      1,
      "/api/me/groups",
      TEST_COOKIE_HEADER,
    );
    expect(mockApiGet).toHaveBeenNthCalledWith(
      2,
      "/api/students?group_id=12",
      TEST_COOKIE_HEADER,
    );
    expect(mockApiGet).toHaveBeenNthCalledWith(
      3,
      "/api/groups/12/students/room-status",
      TEST_COOKIE_HEADER,
    );
    expect(mockApiGet).toHaveBeenNthCalledWith(
      4,
      "/api/groups/12/substitutions",
      TEST_COOKIE_HEADER,
    );

    const json = await parseJsonResponse<
      ApiResponse<{
        groups: typeof groups;
        students: typeof students;
        roomStatus: typeof roomStatus;
        substitutions: typeof substitutions;
        firstGroupId: string | null;
      }>
    >(response);

    expect(json.data.groups).toEqual(groups);
    expect(json.data.students).toEqual(students);
    expect(json.data.roomStatus).toEqual(roomStatus);
    expect(json.data.substitutions).toEqual(substitutions);
    expect(json.data.firstGroupId).toBe("12");
  });

  it("falls back to defaults when parallel fetches fail", async () => {
    const groups = [{ id: 5, name: "OGS B" }];

    mockApiGet
      .mockResolvedValueOnce({ data: groups })
      .mockRejectedValueOnce(new Error("Students failed"))
      .mockRejectedValueOnce(new Error("Room status failed"))
      .mockRejectedValueOnce(new Error("Substitutions failed"));

    const request = createMockRequest("/api/ogs-dashboard");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<
      ApiResponse<{
        students: unknown[];
        roomStatus: unknown;
        substitutions: unknown[];
        firstGroupId: string | null;
      }>
    >(response);

    expect(json.data.students).toEqual([]);
    expect(json.data.roomStatus).toBeNull();
    expect(json.data.substitutions).toEqual([]);
    expect(json.data.firstGroupId).toBe("5");
  });
});
