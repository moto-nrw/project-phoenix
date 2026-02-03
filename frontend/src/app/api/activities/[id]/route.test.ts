import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, PUT, DELETE } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiGet, mockApiPut, mockApiDelete } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPut: vi.fn(),
  mockApiDelete: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: mockApiPut,
  apiDelete: mockApiDelete,
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

describe("GET /api/activities/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("fetches activity by ID with wrapped response", async () => {
    const mockActivity = {
      id: 5,
      name: "Drama Club",
      max_participant: 25,
      is_open_ags: true,
      supervisor_id: 8,
      ag_category_id: 2,
      created_at: "2024-01-15T10:00:00Z",
      updated_at: "2024-01-15T10:00:00Z",
      participant_count: 15,
      times: [],
      students: [],
    };

    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: mockActivity,
    });

    const request = createMockRequest("/api/activities/5");
    const response = await GET(request, createMockContext({ id: "5" }));

    expect(mockApiGet).toHaveBeenCalledWith("/api/activities/5", "test-token");
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: { id: string; name: string };
    }>(response);
    expect(json.data.id).toBe("5");
    expect(json.data.name).toBe("Drama Club");
  });

  it("fetches activity by ID with direct response", async () => {
    const mockActivity = {
      id: 7,
      name: "Science Lab",
      max_participant: 12,
      is_open_ags: false,
      supervisor_id: 3,
      ag_category_id: 1,
      created_at: "2024-01-15T10:00:00Z",
      updated_at: "2024-01-15T10:00:00Z",
      participant_count: 10,
      times: [],
      students: [],
    };

    mockApiGet.mockResolvedValueOnce(mockActivity);

    const request = createMockRequest("/api/activities/7");
    const response = await GET(request, createMockContext({ id: "7" }));

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: { id: string };
    }>(response);
    expect(json.data.id).toBe("7");
  });

  it("returns 404 when activity not found", async () => {
    mockApiGet.mockRejectedValueOnce(
      new Error("API error (404): Activity not found"),
    );

    const request = createMockRequest("/api/activities/999");
    const response = await GET(request, createMockContext({ id: "999" }));

    expect(response.status).toBe(404);
  });

  it("throws error for unexpected response structure", async () => {
    mockApiGet.mockResolvedValueOnce({ unexpected: "structure" });

    const request = createMockRequest("/api/activities/5");
    const response = await GET(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
  });
});

describe("PUT /api/activities/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/5", {
      method: "PUT",
      body: { name: "Updated Name" },
    });
    const response = await PUT(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(401);
  });

  it("updates activity successfully with wrapped response", async () => {
    const updateBody = {
      name: "Updated Chess Club",
      max_participants: 25,
    };

    const mockUpdatedActivity = {
      id: 10,
      name: "Updated Chess Club",
      max_participant: 25,
      is_open_ags: true,
      supervisor_id: 5,
      ag_category_id: 1,
      created_at: "2024-01-15T10:00:00Z",
      updated_at: "2024-01-16T14:00:00Z",
      participant_count: 12,
      times: [],
      students: [],
    };

    mockApiPut.mockResolvedValueOnce({
      status: "success",
      data: mockUpdatedActivity,
    });

    const request = createMockRequest("/api/activities/10", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "10" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/activities/10",
      "test-token",
      updateBody,
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: { id: string; name: string };
    }>(response);
    expect(json.data.id).toBe("10");
    expect(json.data.name).toBe("Updated Chess Club");
  });

  it("updates activity successfully with direct response", async () => {
    const updateBody = { name: "Updated Name" };

    const mockUpdatedActivity = {
      id: 8,
      name: "Updated Name",
      max_participant: 20,
      is_open_ags: false,
      supervisor_id: 2,
      ag_category_id: 3,
      created_at: "2024-01-15T10:00:00Z",
      updated_at: "2024-01-16T14:00:00Z",
      participant_count: 18,
      times: [],
      students: [],
    };

    mockApiPut.mockResolvedValueOnce(mockUpdatedActivity);

    const request = createMockRequest("/api/activities/8", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "8" }));

    expect(response.status).toBe(200);
  });

  it("throws error for invalid activity ID", async () => {
    const request = createMockRequest("/api/activities/invalid", {
      method: "PUT",
      body: { name: "Test" },
    });
    const response = await PUT(request, createMockContext({ id: "invalid" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Invalid activity ID");
  });
});

describe("DELETE /api/activities/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/5", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(401);
  });

  it("deletes activity successfully with 204 status", async () => {
    mockApiDelete.mockResolvedValueOnce({ status: 204 });

    const request = createMockRequest("/api/activities/10", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "10" }));

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/activities/10",
      "test-token",
    );
    // Handler returns { success: true }, gets wrapped and returns 200
    expect([200, 204]).toContain(response.status);
  });

  it("deletes activity successfully with success flag", async () => {
    mockApiDelete.mockResolvedValueOnce({ success: true });

    const request = createMockRequest("/api/activities/15", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "15" }));

    expect([200, 204]).toContain(response.status);
  });

  it("deletes activity successfully with status: success", async () => {
    mockApiDelete.mockResolvedValueOnce({ status: "success" });

    const request = createMockRequest("/api/activities/20", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "20" }));

    expect([200, 204]).toContain(response.status);
  });

  it("throws error for invalid activity ID", async () => {
    const request = createMockRequest("/api/activities/invalid", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "invalid" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Invalid activity ID");
  });

  it("handles empty response from backend", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/activities/25", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "25" }));

    expect([200, 204]).toContain(response.status);
  });
});
