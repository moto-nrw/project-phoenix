import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, POST } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockApiGet, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
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

describe("GET /api/activities/[id]/add-students", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/1/add-students");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("returns eligible students with wrapped response", async () => {
    const students = [
      { id: 1, first_name: "John", last_name: "Doe" },
      { id: 2, first_name: "Jane", last_name: "Smith" },
    ];
    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: students,
    });

    const request = createMockRequest("/api/activities/1/add-students");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/activities/1/eligible-students",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof students>>(response);
    expect(json.data).toEqual(students);
  });

  it("returns eligible students with direct array response", async () => {
    const students = [{ id: 1, first_name: "John", last_name: "Doe" }];
    mockApiGet.mockResolvedValueOnce(students);

    const request = createMockRequest("/api/activities/1/add-students");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof students>>(response);
    expect(json.data).toEqual(students);
  });

  it("returns empty array on error", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("API error"));

    const request = createMockRequest("/api/activities/1/add-students");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });

  it("returns empty array for unexpected response structure", async () => {
    mockApiGet.mockResolvedValueOnce({ unexpected: "structure" });

    const request = createMockRequest("/api/activities/1/add-students");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });
});

describe("POST /api/activities/[id]/add-students", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/1/add-students", {
      method: "POST",
      body: { student_ids: [1, 2, 3] },
    });
    const response = await POST(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("adds students successfully with count from response", async () => {
    mockApiPost.mockResolvedValueOnce({ count: 3 });

    const request = createMockRequest("/api/activities/1/add-students", {
      method: "POST",
      body: { student_ids: [1, 2, 3] },
    });
    const response = await POST(request, createMockContext({ id: "1" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/activities/1/students/batch",
      "test-token",
      { student_ids: [1, 2, 3] },
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{ success: boolean; count: number }>(
      response,
    );
    expect(json.success).toBe(true);
    expect(json.count).toBe(3);
  });

  it("adds students successfully with generic response", async () => {
    mockApiPost.mockResolvedValueOnce({});

    const request = createMockRequest("/api/activities/1/add-students", {
      method: "POST",
      body: { student_ids: [1, 2] },
    });
    const response = await POST(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{ success: boolean; count: number }>(
      response,
    );
    expect(json.success).toBe(true);
    expect(json.count).toBe(2);
  });

  it("converts string student IDs to numbers", async () => {
    mockApiPost.mockResolvedValueOnce({ count: 2 });

    const request = createMockRequest("/api/activities/1/add-students", {
      method: "POST",
      body: { student_ids: ["1", "2"] },
    });
    const response = await POST(request, createMockContext({ id: "1" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/activities/1/students/batch",
      "test-token",
      { student_ids: [1, 2] },
    );
    expect(response.status).toBe(200);
  });

  it("handles errors during batch add", async () => {
    mockApiPost.mockRejectedValueOnce(new Error("Batch add failed"));

    const request = createMockRequest("/api/activities/1/add-students", {
      method: "POST",
      body: { student_ids: [1, 2] },
    });
    const response = await POST(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Batch add failed");
  });
});
