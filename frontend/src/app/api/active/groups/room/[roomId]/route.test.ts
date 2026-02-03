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

function createMockRequest(
  path: string,
  options: { method?: string; body?: unknown } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const requestInit: { method: string; body?: string; headers?: HeadersInit } =
    {
      method: options.method ?? "GET",
    };

  if (options.body) {
    requestInit.body = JSON.stringify(options.body);
    requestInit.headers = { "Content-Type": "application/json" };
  }

  return new NextRequest(url, requestInit);
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

describe("GET /api/active/groups/room/[roomId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/groups/room/789");
    const response = await GET(request, createMockContext({ roomId: "789" }));

    expect(response.status).toBe(401);
  });

  it("fetches active groups in a specific room", async () => {
    const mockActiveGroups = [
      {
        id: 1,
        name: "OGS Group A",
        room_id: 789,
        supervisors: [{ id: 1, name: "Jane Doe" }],
      },
      {
        id: 2,
        name: "OGS Group B",
        room_id: 789,
        supervisors: [{ id: 2, name: "John Smith" }],
      },
    ];
    mockApiGet.mockResolvedValueOnce(mockActiveGroups);

    const request = createMockRequest("/api/active/groups/room/789");
    const response = await GET(request, createMockContext({ roomId: "789" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/groups/room/789",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockActiveGroups>>(response);
    expect(json.data).toEqual(mockActiveGroups);
    expect(json.data).toHaveLength(2);
  });

  it("returns empty array when no active groups exist in room", async () => {
    mockApiGet.mockResolvedValueOnce([]);

    const request = createMockRequest("/api/active/groups/room/999");
    const response = await GET(request, createMockContext({ roomId: "999" }));

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });

  it("returns 500 when roomId parameter is invalid", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Invalid roomId parameter"));

    const request = createMockRequest("/api/active/groups/room/invalid");
    const response = await GET(
      request,
      createMockContext({ roomId: undefined }),
    );

    expect(response.status).toBe(500);
  });
});
