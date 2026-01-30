import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { POST } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: vi.fn(),
  apiPost: mockApiPost,
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
      method: options.method ?? "POST",
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

describe("POST /api/active/groups/[id]/claim", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/groups/123/claim", {
      method: "POST",
    });
    const response = await POST(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("claims supervision of an active group", async () => {
    const mockClaimedGroup = {
      id: 123,
      name: "Schulhof Group",
      supervisors: [{ id: 1, name: "Test User", role: "supervisor" }],
    };
    mockApiPost.mockResolvedValueOnce(mockClaimedGroup);

    const request = createMockRequest("/api/active/groups/123/claim", {
      method: "POST",
    });
    const response = await POST(request, createMockContext({ id: "123" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/active/groups/123/claim",
      "test-token",
      { role: "supervisor" },
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockClaimedGroup>>(response);
    expect(json.data).toEqual(mockClaimedGroup);
  });

  it("returns 404 when group not found", async () => {
    mockApiPost.mockRejectedValueOnce(new Error("Group not found (404)"));

    const request = createMockRequest("/api/active/groups/999/claim", {
      method: "POST",
    });
    const response = await POST(request, createMockContext({ id: "999" }));

    expect(response.status).toBe(404);
  });

  it("returns 500 when id parameter is invalid", async () => {
    mockApiPost.mockRejectedValueOnce(new TypeError("Invalid group ID"));

    const request = createMockRequest("/api/active/groups/invalid/claim", {
      method: "POST",
    });
    const response = await POST(request, createMockContext({ id: undefined }));

    expect(response.status).toBe(500);
  });

  it("automatically sets role to supervisor", async () => {
    mockApiPost.mockResolvedValueOnce({ id: 123 });

    const request = createMockRequest("/api/active/groups/123/claim", {
      method: "POST",
      body: {},
    });
    await POST(request, createMockContext({ id: "123" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/active/groups/123/claim",
      "test-token",
      { role: "supervisor" },
    );
  });
});
