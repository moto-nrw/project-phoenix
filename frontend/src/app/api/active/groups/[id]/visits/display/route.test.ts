import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET } from "./route";
import { ApiResponseError } from "~/lib/api-response-error.server";

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

vi.mock("~/lib/api-helpers.server", async () => {
  const { mockHandleApiError } =
    await import("~/test/helpers/handle-api-error");
  return {
    apiGet: mockApiGet,
    apiPost: vi.fn(),
    apiPut: vi.fn(),
    apiDelete: vi.fn(),
    handleApiError: vi.fn(mockHandleApiError),
  };
});

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

describe("GET /api/active/groups/[id]/visits/display", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/groups/1/visits/display");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("returns 500 when id parameter is missing", async () => {
    const request = createMockRequest(
      "/api/active/groups/undefined/visits/display",
    );
    const response = await GET(request, createMockContext({}));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Invalid id parameter");
  });

  it("returns 500 when id parameter is not a string", async () => {
    const request = createMockRequest("/api/active/groups/1/visits/display");
    const response = await GET(request, createMockContext({ id: ["1", "2"] }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Invalid id parameter");
  });

  it("fetches visits with display data for valid group id", async () => {
    const mockVisits = [
      {
        id: 1,
        student_id: 101,
        active_group_id: 1,
        start_time: "2024-01-15T09:00:00Z",
        end_time: null,
        student: {
          id: 101,
          first_name: "Max",
          second_name: "Mustermann",
        },
      },
      {
        id: 2,
        student_id: 102,
        active_group_id: 1,
        start_time: "2024-01-15T09:15:00Z",
        end_time: null,
        student: {
          id: 102,
          first_name: "Maria",
          second_name: "Schmidt",
        },
      },
    ];
    mockApiGet.mockResolvedValueOnce({ data: mockVisits });

    const request = createMockRequest("/api/active/groups/1/visits/display");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/groups/1/visits/display",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockVisits>>(response);
    expect(json.data).toEqual(mockVisits);
  });

  it("returns empty array when group has no visits", async () => {
    mockApiGet.mockResolvedValueOnce({ data: [] });

    const request = createMockRequest("/api/active/groups/1/visits/display");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });

  it("handles API errors gracefully", async () => {
    mockApiGet.mockRejectedValueOnce(
      new ApiResponseError(404, "Group not found"),
    );

    const request = createMockRequest("/api/active/groups/999/visits/display");
    const response = await GET(request, createMockContext({ id: "999" }));

    expect(response.status).toBe(404);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Not Found");
    expect(JSON.stringify(json)).not.toContain("Group not found");
  });
});
