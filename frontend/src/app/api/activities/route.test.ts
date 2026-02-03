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
// Mocks (using vi.hoisted for proper hoisting)
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

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/activities", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("returns paginated activities with status: success wrapper", async () => {
    const mockActivities = [
      {
        id: 1,
        name: "Chess Club",
        max_participant: 20,
        is_open_ags: true,
        supervisor_id: 5,
        ag_category_id: 1,
        created_at: "2024-01-15T10:00:00Z",
        updated_at: "2024-01-15T10:00:00Z",
        participant_count: 12,
        times: [],
        students: [],
      },
    ];

    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: mockActivities,
    });

    const request = createMockRequest("/api/activities");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/activities?page_size=1000",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: {
        data: Array<{ id: string; name: string }>;
        pagination: { total_records: number };
      };
    }>(response);

    expect(json.data.data).toHaveLength(1);
    expect(json.data.data[0]!.id).toBe("1");
    expect(json.data.data[0]!.name).toBe("Chess Club");
    expect(json.data.pagination.total_records).toBe(1);
  });

  it("handles direct array response", async () => {
    const mockActivities = [
      {
        id: 2,
        name: "Soccer",
        max_participant: 22,
        is_open_ags: false,
        supervisor_id: 3,
        ag_category_id: 2,
        created_at: "2024-01-15T10:00:00Z",
        updated_at: "2024-01-15T10:00:00Z",
        participant_count: 18,
        times: [],
        students: [],
      },
    ];

    mockApiGet.mockResolvedValueOnce(mockActivities);

    const request = createMockRequest("/api/activities");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: {
        data: Array<{ id: string }>;
      };
    }>(response);

    expect(json.data.data).toHaveLength(1);
    expect(json.data.data[0]!.id).toBe("2");
  });

  it("returns empty array for unexpected response structure", async () => {
    mockApiGet.mockResolvedValueOnce({ unexpected: "structure" });

    const request = createMockRequest("/api/activities");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<{
      success: boolean;
      data: {
        data: Array<unknown>;
        pagination: { total_records: number };
      };
    }>(response);

    expect(json.data.data).toEqual([]);
    expect(json.data.pagination.total_records).toBe(0);
  });

  it("forwards query parameters to backend", async () => {
    mockApiGet.mockResolvedValueOnce({ status: "success", data: [] });

    const request = createMockRequest(
      "/api/activities?category_id=5&search=chess",
    );
    await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/activities?category_id=5&search=chess&page_size=1000",
      "test-token",
    );
  });
});

describe("POST /api/activities", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities", {
      method: "POST",
      body: { name: "New Activity", max_participants: 10, category_id: 1 },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("creates activity successfully", async () => {
    const requestBody = {
      name: "Art Class",
      max_participants: 15,
      category_id: 3,
    };

    const mockCreatedActivity = {
      id: 10,
      name: "Art Class",
      max_participant: 15,
      is_open_ags: false,
      supervisor_id: 2,
      ag_category_id: 3,
      created_at: "2024-01-15T10:00:00Z",
      updated_at: "2024-01-15T10:00:00Z",
      participant_count: 0,
      times: [],
      students: [],
    };

    mockApiPost.mockResolvedValueOnce({
      status: "success",
      data: mockCreatedActivity,
    });

    const request = createMockRequest("/api/activities", {
      method: "POST",
      body: requestBody,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/activities",
      "test-token",
      requestBody,
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: { id: string; name: string };
    }>(response);
    expect(json.data.id).toBe("10");
    expect(json.data.name).toBe("Art Class");
  });

  it("validates required name field", async () => {
    const request = createMockRequest("/api/activities", {
      method: "POST",
      body: { name: "", max_participants: 10, category_id: 1 },
    });

    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Name is required");
  });

  it("validates max_participants must be greater than 0", async () => {
    const request = createMockRequest("/api/activities", {
      method: "POST",
      body: { name: "Test", max_participants: 0, category_id: 1 },
    });

    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Max participants must be greater than 0");
  });

  it("validates category_id is required", async () => {
    const request = createMockRequest("/api/activities", {
      method: "POST",
      body: { name: "Test", max_participants: 10 },
    });

    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Category is required");
  });

  it("returns fallback activity when backend returns unexpected structure", async () => {
    const requestBody = {
      name: "Music Club",
      max_participants: 12,
      category_id: 4,
    };

    mockApiPost.mockResolvedValueOnce({ unexpected: "structure" });

    const request = createMockRequest("/api/activities", {
      method: "POST",
      body: requestBody,
    });
    const response = await POST(request, createMockContext());

    const json = await parseJsonResponse<{
      success: boolean;
      data: { name: string; id: string };
    }>(response);
    expect(json.data.name).toBe("Music Club");
    expect(json.data.id).toBe("0");
  });
});
