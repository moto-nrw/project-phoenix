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

describe("POST /api/active/supervisors/[id]/end", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/supervisors/1/end");
    const response = await POST(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("ends supervision for a supervisor", async () => {
    const mockEndedSupervisor = {
      id: 1,
      staff_id: 10,
      active_group_id: 5,
      ended_at: "2024-01-15T17:00:00Z",
    };
    mockApiPost.mockResolvedValueOnce(mockEndedSupervisor);

    const request = createMockRequest("/api/active/supervisors/1/end");
    const response = await POST(request, createMockContext({ id: "1" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/active/supervisors/1/end",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockEndedSupervisor>>(
        response,
      );
    expect(json.data).toEqual(mockEndedSupervisor);
  });

  it("returns error when id is invalid", async () => {
    const request = createMockRequest("/api/active/supervisors/invalid/end");
    const response = await POST(request, createMockContext({ id: undefined }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Invalid id parameter");
  });

  it("returns error when id is not a string", async () => {
    const request = createMockRequest("/api/active/supervisors/1/end");
    const response = await POST(request, createMockContext({ id: ["1", "2"] }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Invalid id parameter");
  });
});
