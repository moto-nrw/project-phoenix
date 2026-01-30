import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { POST } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

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

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("POST /api/activities/quick-create", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/quick-create", {
      method: "POST",
      body: { name: "Quick Activity", category_id: 1, max_participants: 10 },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("creates activity quickly with wrapped response", async () => {
    const requestBody = {
      name: "Quick Chess",
      category_id: 1,
      max_participants: 12,
      room_id: 5,
    };

    const mockResponse = {
      activity_id: 100,
      name: "Quick Chess",
      category_name: "Board Games",
      room_name: "Room A",
      supervisor_name: "Mr. Smith",
      status: "created",
      message: "Activity created successfully",
      created_at: "2024-01-20T10:00:00Z",
    };

    mockApiPost.mockResolvedValueOnce({
      status: "success",
      data: mockResponse,
    });

    const request = createMockRequest("/api/activities/quick-create", {
      method: "POST",
      body: requestBody,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/activities/quick-create",
      "test-token",
      requestBody,
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: {
        activity_id: number;
        name: string;
        status: string;
      };
    }>(response);
    expect(json.data.activity_id).toBe(100);
    expect(json.data.name).toBe("Quick Chess");
    expect(json.data.status).toBe("created");
  });

  it("creates activity quickly with direct response", async () => {
    const requestBody = {
      name: "Quick Soccer",
      category_id: 2,
      max_participants: 20,
    };

    const mockResponse = {
      activity_id: 101,
      name: "Quick Soccer",
      category_name: "Sports",
      supervisor_name: "Ms. Johnson",
      status: "created",
      message: "Success",
      created_at: "2024-01-20T10:00:00Z",
    };

    mockApiPost.mockResolvedValueOnce(mockResponse);

    const request = createMockRequest("/api/activities/quick-create", {
      method: "POST",
      body: requestBody,
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: { activity_id: number };
    }>(response);
    expect(json.data.activity_id).toBe(101);
  });

  it("validates activity name is required", async () => {
    const request = createMockRequest("/api/activities/quick-create", {
      method: "POST",
      body: { name: "", category_id: 1, max_participants: 10 },
    });

    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Activity name is required");
  });

  it("validates category_id is required", async () => {
    const request = createMockRequest("/api/activities/quick-create", {
      method: "POST",
      body: { name: "Test", max_participants: 10 },
    });

    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Valid category is required");
  });

  it("validates category_id must be greater than 0", async () => {
    const request = createMockRequest("/api/activities/quick-create", {
      method: "POST",
      body: { name: "Test", category_id: 0, max_participants: 10 },
    });

    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Valid category is required");
  });

  it("validates max_participants is required", async () => {
    const request = createMockRequest("/api/activities/quick-create", {
      method: "POST",
      body: { name: "Test", category_id: 1 },
    });

    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Max participants must be greater than 0");
  });

  it("returns fallback response when backend returns unexpected structure", async () => {
    const requestBody = {
      name: "Fallback Activity",
      category_id: 3,
      max_participants: 15,
    };

    mockApiPost.mockResolvedValueOnce({ unexpected: "structure" });

    const request = createMockRequest("/api/activities/quick-create", {
      method: "POST",
      body: requestBody,
    });
    const response = await POST(request, createMockContext());

    const json = await parseJsonResponse<{
      success: boolean;
      data: {
        activity_id: number;
        name: string;
        status: string;
      };
    }>(response);
    expect(json.data.activity_id).toBe(0);
    expect(json.data.name).toBe("Fallback Activity");
    expect(json.data.status).toBe("created");
  });
});
