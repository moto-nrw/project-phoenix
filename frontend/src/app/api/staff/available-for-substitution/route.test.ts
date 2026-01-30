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
// Mocks (using vi.hoisted for proper hoisting)
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

describe("GET /api/staff/available-for-substitution", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/staff/available-for-substitution");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches available staff without query parameters", async () => {
    const mockStaff = {
      status: "success",
      data: [
        {
          id: 1,
          person_id: 100,
          is_teacher: true,
          role: "Teacher",
          person: {
            first_name: "John",
            last_name: "Doe",
          },
          current_status: "available",
        },
      ],
      message: "Available staff retrieved",
    };
    mockApiGet.mockResolvedValueOnce(mockStaff);

    const request = createMockRequest("/api/staff/available-for-substitution");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/staff/available-for-substitution",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toHaveLength(1);
  });

  it("fetches available staff with date filter", async () => {
    const mockStaff = {
      status: "success",
      data: [],
      message: "No available staff",
    };
    mockApiGet.mockResolvedValueOnce(mockStaff);

    const request = createMockRequest(
      "/api/staff/available-for-substitution?date=2024-01-15",
    );
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/staff/available-for-substitution?date=2024-01-15",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("fetches available staff with search filter", async () => {
    const mockStaff = {
      status: "success",
      data: [
        {
          id: 1,
          person_id: 100,
          person: { first_name: "John", last_name: "Doe" },
        },
      ],
      message: "Filtered staff",
    };
    mockApiGet.mockResolvedValueOnce(mockStaff);

    const request = createMockRequest(
      "/api/staff/available-for-substitution?search=John",
    );
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/staff/available-for-substitution?search=John",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("fetches available staff with both date and search filters", async () => {
    const mockStaff = {
      status: "success",
      data: [],
      message: "No matching staff",
    };
    mockApiGet.mockResolvedValueOnce(mockStaff);

    const request = createMockRequest(
      "/api/staff/available-for-substitution?date=2024-01-15&search=John",
    );
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/staff/available-for-substitution?date=2024-01-15&search=John",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("returns empty array when response data is null", async () => {
    mockApiGet.mockResolvedValueOnce({ status: "success", data: null });

    const request = createMockRequest("/api/staff/available-for-substitution");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });
});
