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
    // Match the real handleApiError regex pattern
    const regex = /API error[:\s(]+(\d{3})/;
    const match = error instanceof Error ? regex.exec(error.message) : null;
    const status = match?.[1] ? Number.parseInt(match[1], 10) : 500;
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

describe("GET /api/active/visits/student/[studentId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/visits/student/123");
    const response = await GET(
      request,
      createMockContext({ studentId: "123" }),
    );

    expect(response.status).toBe(401);
  });

  it("returns 500 when studentId parameter is missing", async () => {
    const request = createMockRequest("/api/active/visits/student/");

    const response = await GET(request, createMockContext({}));

    expect(response.status).toBe(500);
    expect(mockApiGet).not.toHaveBeenCalled();
  });

  it("returns 500 when studentId parameter is not a string", async () => {
    const request = createMockRequest("/api/active/visits/student/123");

    const response = await GET(
      request,
      createMockContext({ studentId: ["123", "456"] }),
    );

    expect(response.status).toBe(500);
    expect(mockApiGet).not.toHaveBeenCalled();
  });

  it("fetches student visits from backend", async () => {
    const mockVisits = [
      {
        id: 1,
        student_id: 123,
        active_group_id: 1,
        start_time: "2024-01-15T09:00:00Z",
      },
      {
        id: 2,
        student_id: 123,
        active_group_id: 2,
        start_time: "2024-01-15T10:00:00Z",
      },
    ];
    mockApiGet.mockResolvedValueOnce(mockVisits);

    const request = createMockRequest("/api/active/visits/student/123");
    const response = await GET(
      request,
      createMockContext({ studentId: "123" }),
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/visits/student/123",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockVisits>>(response);
    expect(json.data).toEqual(mockVisits);
  });

  it("handles backend errors gracefully", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Backend error (500)"));

    const request = createMockRequest("/api/active/visits/student/123");
    const response = await GET(
      request,
      createMockContext({ studentId: "123" }),
    );

    expect(response.status).toBe(500);
  });
});
