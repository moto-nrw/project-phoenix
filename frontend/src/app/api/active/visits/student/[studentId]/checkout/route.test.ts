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

const { mockAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

// Mock global fetch
global.fetch = mockFetch as unknown as typeof fetch;

vi.mock("~/lib/api-helpers", () => ({
  apiGet: vi.fn(),
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

describe("POST /api/active/visits/student/[studentId]/checkout", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest(
      "/api/active/visits/student/123/checkout",
      { method: "POST", body: {} },
    );
    const response = await POST(
      request,
      createMockContext({ studentId: "123" }),
    );

    expect(response.status).toBe(401);
  });

  it("returns 500 when studentId parameter is missing", async () => {
    const request = createMockRequest("/api/active/visits/student//checkout", {
      method: "POST",
      body: {},
    });

    const response = await POST(request, createMockContext({}));

    expect(response.status).toBe(500);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("successfully checks out student", async () => {
    const mockResponse = {
      status: "success",
      message: "Student checked out successfully",
      data: {
        id: 123,
        student_id: 456,
        end_time: "2024-01-15T17:00:00Z",
      },
    };

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => mockResponse,
    } as Response);

    const request = createMockRequest(
      "/api/active/visits/student/456/checkout",
      { method: "POST", body: {} },
    );
    const response = await POST(
      request,
      createMockContext({ studentId: "456" }),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/active/visits/student/456/checkout"),
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockResponse.data>>(response);
    expect(json.data).toEqual(mockResponse.data);
  });

  it("handles backend errors gracefully", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      text: async () => "Student not found",
    } as Response);

    const request = createMockRequest(
      "/api/active/visits/student/999/checkout",
      { method: "POST", body: {} },
    );

    const response = await POST(
      request,
      createMockContext({ studentId: "999" }),
    );

    expect(response.status).toBe(500);
  });

  it("handles empty error response", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      text: async () => "",
    } as Response);

    const request = createMockRequest(
      "/api/active/visits/student/999/checkout",
      { method: "POST", body: {} },
    );

    const response = await POST(
      request,
      createMockContext({ studentId: "999" }),
    );

    expect(response.status).toBe(500);
  });
});
