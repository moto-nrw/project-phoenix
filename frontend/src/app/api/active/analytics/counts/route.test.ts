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

describe("GET /api/active/analytics/counts", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/analytics/counts");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches analytics counts from backend", async () => {
    const mockCounts = {
      active_groups: 5,
      total_visits: 120,
      active_supervisors: 8,
      students_in_house: 85,
    };

    mockApiGet.mockResolvedValueOnce(mockCounts);

    const request = createMockRequest("/api/active/analytics/counts");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/active/analytics/counts",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockCounts>>(response);
    expect(json.data).toEqual(mockCounts);
  });

  it("handles zero counts", async () => {
    const mockCounts = {
      active_groups: 0,
      total_visits: 0,
      active_supervisors: 0,
      students_in_house: 0,
    };

    mockApiGet.mockResolvedValueOnce(mockCounts);

    const request = createMockRequest("/api/active/analytics/counts");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    const json =
      await parseJsonResponse<ApiResponse<typeof mockCounts>>(response);
    expect(json.data).toEqual(mockCounts);
  });

  it("forwards backend errors", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Backend error (500)"));

    const request = createMockRequest("/api/active/analytics/counts");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
  });
});
