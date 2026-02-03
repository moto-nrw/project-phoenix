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

describe("GET /api/active/supervisors/staff/[staffId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/supervisors/staff/10");
    const response = await GET(request, createMockContext({ staffId: "10" }));

    expect(response.status).toBe(401);
  });

  it("fetches supervisions for a staff member", async () => {
    const mockSupervisions = [
      { id: 1, staff_id: 10, active_group_id: 5 },
      { id: 2, staff_id: 10, active_group_id: 6 },
    ];
    mockApiGet.mockResolvedValueOnce(mockSupervisions);

    const request = createMockRequest("/api/active/supervisors/staff/10");
    const response = await GET(request, createMockContext({ staffId: "10" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/supervisors/staff/10",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockSupervisions>>(response);
    expect(json.data).toEqual(mockSupervisions);
  });

  it("returns error when staffId is invalid", async () => {
    const request = createMockRequest("/api/active/supervisors/staff/invalid");
    const response = await GET(
      request,
      createMockContext({ staffId: undefined }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Invalid staffId parameter");
  });

  it("returns error when staffId is not a string", async () => {
    const request = createMockRequest("/api/active/supervisors/staff/10");
    const response = await GET(
      request,
      createMockContext({ staffId: ["10", "11"] }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Invalid staffId parameter");
  });
});
