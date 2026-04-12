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
// Mocks
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

function createMockRequest(body: unknown): NextRequest {
  const url = new URL(
    "/api/active/tracking-indicators",
    "http://localhost:3000",
  );
  return new NextRequest(url, {
    method: "POST",
    body: JSON.stringify(body),
    headers: { "Content-Type": "application/json" },
  });
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

describe("POST /api/active/tracking-indicators", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest({ student_ids: [10, 20] });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("proxies request to backend and returns data", async () => {
    const innerData = {
      labels: ["Hausaufgaben", "Mensa"],
      results: { "10": [true, false], "20": [false, true] },
    };

    // apiPost returns the full response; handler accesses .data
    mockApiPost.mockResolvedValueOnce({ data: innerData });

    const request = createMockRequest({ student_ids: [10, 20] });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(200);
    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/active/tracking-indicators",
      "test-token",
      { student_ids: [10, 20] },
    );

    const json =
      await parseJsonResponse<ApiResponse<typeof innerData>>(response);
    expect(json.data).toEqual(innerData);
  });

  it("returns empty labels and results when feature is disabled", async () => {
    const innerData = {
      labels: [] as string[],
      results: {} as Record<string, boolean[]>,
    };

    mockApiPost.mockResolvedValueOnce({ data: innerData });

    const request = createMockRequest({ student_ids: [10] });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(200);
    const json =
      await parseJsonResponse<ApiResponse<typeof innerData>>(response);
    expect(json.data.labels).toEqual([]);
    expect(json.data.results).toEqual({});
  });

  it("forwards backend errors", async () => {
    mockApiPost.mockRejectedValueOnce(new Error("Backend error (500)"));

    const request = createMockRequest({ student_ids: [10] });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
  });
});
