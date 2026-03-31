import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { PUT, DELETE } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockApiPut, mockApiDelete, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPut: vi.fn(),
  mockApiDelete: vi.fn(),
  mockApiGet: vi.fn(),
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

describe("PUT /api/activities/[id]/supervisors/[supervisorId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/1/supervisors/2", {
      method: "PUT",
      body: { is_primary: true },
    });
    const response = await PUT(
      request,
      createMockContext({ id: "1", supervisorId: "2" }),
    );

    expect(response.status).toBe(401);
  });

  it("updates supervisor role successfully", async () => {
    const updatedSupervisors = [{ id: 2, staff_id: 10, is_primary: true }];
    mockApiPut.mockResolvedValueOnce(undefined);
    mockApiGet.mockResolvedValueOnce({ data: updatedSupervisors });

    const request = createMockRequest("/api/activities/1/supervisors/2", {
      method: "PUT",
      body: { is_primary: true },
    });
    const response = await PUT(
      request,
      createMockContext({ id: "1", supervisorId: "2" }),
    );

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/activities/1/supervisors/2",
      "test-token",
      { is_primary: true },
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof updatedSupervisors>>(response);
    expect(json.data).toEqual(updatedSupervisors);
  });

  it("uses the numeric id extracted from the URL when activityId context is empty", async () => {
    mockApiPut.mockRejectedValueOnce(
      new Error("Fallback activity id was extracted from URL"),
    );

    const request = createMockRequest("/api/activities//supervisors/2", {
      method: "PUT",
      body: { is_primary: true },
    });
    const response = await PUT(
      request,
      createMockContext({ id: "", supervisorId: "2" }),
    );

    expect(response.status).toBe(500);
    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/activities/2/supervisors/2",
      "test-token",
      { is_primary: true },
    );
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Fallback activity id was extracted from URL");
  });

  it("returns 400 when supervisorId is missing", async () => {
    const request = createMockRequest("/api/activities/1/supervisors/", {
      method: "PUT",
      body: { is_primary: true },
    });
    const response = await PUT(
      request,
      createMockContext({ id: "1", supervisorId: "" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Supervisor ID is required");
  });

  it("returns 400 when is_primary is missing", async () => {
    const request = createMockRequest("/api/activities/1/supervisors/2", {
      method: "PUT",
      body: {},
    });
    const response = await PUT(
      request,
      createMockContext({ id: "1", supervisorId: "2" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("is_primary parameter is required");
  });

  it("returns 500 when update fails", async () => {
    mockApiPut.mockRejectedValueOnce(
      new Error("Failed to update supervisor role"),
    );

    const request = createMockRequest("/api/activities/1/supervisors/2", {
      method: "PUT",
      body: { is_primary: true },
    });
    const response = await PUT(
      request,
      createMockContext({ id: "1", supervisorId: "2" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Failed to update supervisor role");
  });
});

describe("DELETE /api/activities/[id]/supervisors/[supervisorId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/1/supervisors/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "1", supervisorId: "2" }),
    );

    expect(response.status).toBe(401);
  });

  it("removes supervisor successfully", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/activities/1/supervisors/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "1", supervisorId: "2" }),
    );

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/activities/1/supervisors/2",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{ success: boolean }>(response);
    expect(json.success).toBe(true);
  });

  it("uses the numeric id extracted from the URL when activityId context is empty", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/activities//supervisors/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "", supervisorId: "2" }),
    );

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/activities/2/supervisors/2",
      "test-token",
    );
    expect(response.status).toBe(200);
    const json = await parseJsonResponse<{ success: boolean }>(response);
    expect(json.success).toBe(true);
  });

  it("returns 400 when supervisorId is missing", async () => {
    const request = createMockRequest("/api/activities/1/supervisors/", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "1", supervisorId: "" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Supervisor ID is required");
  });

  it("returns 500 when removal fails", async () => {
    mockApiDelete.mockRejectedValueOnce(
      new Error("Failed to remove supervisor"),
    );

    const request = createMockRequest("/api/activities/1/supervisors/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "1", supervisorId: "2" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Failed to remove supervisor");
  });
});
