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

describe("GET /api/active/groups/unclaimed", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/groups/unclaimed");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches all unclaimed active groups", async () => {
    const mockUnclaimedGroups = [
      {
        id: 1,
        name: "Schulhof Group A",
        room_id: 100,
        supervisors: [],
      },
      {
        id: 2,
        name: "Schulhof Group B",
        room_id: 101,
        supervisors: [],
      },
    ];
    mockApiGet.mockResolvedValueOnce(mockUnclaimedGroups);

    const request = createMockRequest("/api/active/groups/unclaimed");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/groups/unclaimed",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUnclaimedGroups>>(
        response,
      );
    expect(json.data).toEqual(mockUnclaimedGroups);
    expect(json.data).toHaveLength(2);
  });

  it("returns empty array when no unclaimed groups exist", async () => {
    mockApiGet.mockResolvedValueOnce([]);

    const request = createMockRequest("/api/active/groups/unclaimed");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });

  it("handles backend errors gracefully", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Backend error"));

    const request = createMockRequest("/api/active/groups/unclaimed");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
  });
});
