import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { GET } from "./route";

// ============================================================================
// Mocks
// ============================================================================

const { mockApiGet } = vi.hoisted(() => ({
  mockApiGet: vi.fn(),
}));

vi.mock("~/lib/api-client", () => ({
  apiGet: mockApiGet,
}));

// Note: auth() is globally mocked in setup.ts
// It checks for better-auth.session_token cookie

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

interface DatabaseStats {
  students: number;
  teachers: number;
  rooms: number;
  activities: number;
  groups: number;
  roles: number;
  devices: number;
  permissionCount: number;
  permissions: {
    canViewStudents: boolean;
    canViewTeachers: boolean;
    canViewRooms: boolean;
    canViewActivities: boolean;
    canViewGroups: boolean;
    canViewRoles: boolean;
    canViewDevices: boolean;
    canViewPermissions: boolean;
  };
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/database/counts", () => {
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

    const request = createMockRequest("/api/database/counts");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("returns database stats when authenticated", async () => {
    const mockStats: DatabaseStats = {
      students: 150,
      teachers: 20,
      rooms: 10,
      activities: 30,
      groups: 15,
      roles: 5,
      devices: 8,
      permissionCount: 25,
      permissions: {
        canViewStudents: true,
        canViewTeachers: true,
        canViewRooms: true,
        canViewActivities: true,
        canViewGroups: true,
        canViewRoles: false,
        canViewDevices: false,
        canViewPermissions: false,
      },
    };

    mockApiGet.mockResolvedValueOnce({ data: mockStats });

    const request = createMockRequest("/api/database/counts");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/database/stats",
      TEST_COOKIE_HEADER,
    );

    const json = (await response.json()) as {
      success: boolean;
      data: DatabaseStats;
    };
    expect(json.success).toBe(true);
    expect(json.data).toEqual(mockStats);
  });

  it("handles API errors gracefully", async () => {
    mockApiGet.mockRejectedValueOnce(
      new Error("API error (500): Internal server error"),
    );

    const request = createMockRequest("/api/database/counts");
    const response = await GET(request, createMockContext());

    // handleApiError should return appropriate error response
    expect(response.status).toBe(500);
  });

  it("passes correct endpoint to apiGet", async () => {
    mockApiGet.mockResolvedValueOnce({ data: {} });

    const request = createMockRequest("/api/database/counts");
    await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/database/stats",
      TEST_COOKIE_HEADER,
    );
  });
});
