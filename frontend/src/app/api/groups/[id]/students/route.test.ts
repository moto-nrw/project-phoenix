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

vi.mock("~/lib/api-client", () => ({
  apiGet: mockApiGet,
}));

vi.mock("~/lib/api-helpers", () => ({
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

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/groups/[id]/students", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/groups/123/students");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("returns 400 when group ID is missing", async () => {
    const request = createMockRequest("/api/groups//students");
    const response = await GET(request, createMockContext({}));

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Group ID is required");
  });

  it("fetches students in group from backend", async () => {
    const mockStudents = [
      { id: 1, first_name: "Alice", second_name: "A" },
      { id: 2, first_name: "Bob", second_name: "B" },
    ];
    mockApiGet.mockResolvedValueOnce({ data: mockStudents });

    const request = createMockRequest("/api/groups/123/students");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/groups/123/students",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<typeof mockStudents>(response);
    expect(json).toEqual(mockStudents);
  });

  it("returns array directly if response is already an array", async () => {
    const mockStudents = [{ id: 1, first_name: "Charlie", second_name: "C" }];
    mockApiGet.mockResolvedValueOnce(mockStudents);

    const request = createMockRequest("/api/groups/123/students");
    const response = await GET(request, createMockContext({ id: "123" }));

    const json = await parseJsonResponse<typeof mockStudents>(response);
    expect(json).toEqual(mockStudents);
  });
});
