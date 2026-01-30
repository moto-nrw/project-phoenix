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

describe("GET /api/active/groups/group/[groupId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/groups/group/456");
    const response = await GET(request, createMockContext({ groupId: "456" }));

    expect(response.status).toBe(401);
  });

  it("fetches active groups for a specific education group", async () => {
    const mockActiveGroups = [
      {
        id: 1,
        name: "OGS Group A",
        education_group_id: 456,
        room_id: 10,
      },
      {
        id: 2,
        name: "OGS Group B",
        education_group_id: 456,
        room_id: 11,
      },
    ];
    mockApiGet.mockResolvedValueOnce(mockActiveGroups);

    const request = createMockRequest("/api/active/groups/group/456");
    const response = await GET(request, createMockContext({ groupId: "456" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/groups/group/456",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockActiveGroups>>(response);
    expect(json.data).toEqual(mockActiveGroups);
    expect(json.data).toHaveLength(2);
  });

  it("returns empty array when no active groups exist for education group", async () => {
    mockApiGet.mockResolvedValueOnce([]);

    const request = createMockRequest("/api/active/groups/group/999");
    const response = await GET(request, createMockContext({ groupId: "999" }));

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });

  it("returns 500 when groupId parameter is invalid", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Invalid groupId parameter"));

    const request = createMockRequest("/api/active/groups/group/invalid");
    const response = await GET(
      request,
      createMockContext({ groupId: undefined }),
    );

    expect(response.status).toBe(500);
  });
});
