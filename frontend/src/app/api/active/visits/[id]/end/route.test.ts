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

describe("POST /api/active/visits/[id]/end", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/visits/123/end", {
      method: "POST",
    });
    const response = await POST(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("returns 500 when id parameter is missing", async () => {
    const request = createMockRequest("/api/active/visits//end", {
      method: "POST",
    });

    const response = await POST(request, createMockContext({}));

    expect(response.status).toBe(500);
    expect(mockApiPost).not.toHaveBeenCalled();
  });

  it("returns 500 when id parameter is not a string", async () => {
    const request = createMockRequest("/api/active/visits/123/end", {
      method: "POST",
    });

    const response = await POST(
      request,
      createMockContext({ id: ["123", "456"] }),
    );

    expect(response.status).toBe(500);
    expect(mockApiPost).not.toHaveBeenCalled();
  });

  it("successfully ends a visit", async () => {
    const mockEndedVisit = {
      id: 123,
      student_id: 456,
      active_group_id: 1,
      start_time: "2024-01-15T09:00:00Z",
      end_time: "2024-01-15T17:00:00Z",
    };
    mockApiPost.mockResolvedValueOnce(mockEndedVisit);

    const request = createMockRequest("/api/active/visits/123/end", {
      method: "POST",
    });
    const response = await POST(request, createMockContext({ id: "123" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/active/visits/123/end",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockEndedVisit>>(response);
    expect(json.data).toEqual(mockEndedVisit);
    expect(json.data.end_time).toBe("2024-01-15T17:00:00Z");
  });

  it("handles backend errors gracefully", async () => {
    mockApiPost.mockRejectedValueOnce(new Error("API error (404)"));

    const request = createMockRequest("/api/active/visits/999/end", {
      method: "POST",
    });
    const response = await POST(request, createMockContext({ id: "999" }));

    expect(response.status).toBe(404);
  });

  it("handles visit already ended", async () => {
    mockApiPost.mockRejectedValueOnce(new Error("API error (400)"));

    const request = createMockRequest("/api/active/visits/123/end", {
      method: "POST",
    });
    const response = await POST(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(400);
  });
});
