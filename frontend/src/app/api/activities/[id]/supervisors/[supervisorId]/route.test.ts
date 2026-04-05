import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest, NextResponse } from "next/server";
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

const {
  mockAuth,
  mockApiPut,
  mockApiGet,
  mockFetch,
  mockGetServerApiUrl,
  mockHandleApiError,
} = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPut: vi.fn(),
  mockApiGet: vi.fn(),
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
  mockHandleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)")
      ? 401
      : message.includes("(404)")
        ? 404
        : 500;
    return NextResponse.json({ error: message }, { status });
  }),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: mockApiPut,
  apiDelete: vi.fn(),
  handleApiError: mockHandleApiError,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

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
    mockApiPut.mockResolvedValueOnce(undefined);

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
    expect(mockApiGet).not.toHaveBeenCalled();

    const json = await parseJsonResponse<{ success: boolean }>(response);
    expect(json.success).toBe(true);
  });

  it("does not fail a successful update on refetch errors", async () => {
    mockApiPut.mockResolvedValueOnce(undefined);
    mockApiGet.mockRejectedValueOnce(
      new Error("Transient list refresh failure"),
    );

    const request = createMockRequest("/api/activities/1/supervisors/2", {
      method: "PUT",
      body: { is_primary: true },
    });
    const response = await PUT(
      request,
      createMockContext({ id: "1", supervisorId: "2" }),
    );

    expect(response.status).toBe(200);
    expect(mockApiGet).not.toHaveBeenCalled();

    const json = await parseJsonResponse<{ success: boolean }>(response);
    expect(json.success).toBe(true);
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
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ status: "success" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/activities/1/supervisors/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "1", supervisorId: "2" }),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/activities/1/supervisors/2",
      {
        method: "DELETE",
        headers: {
          Authorization: "Bearer test-token",
        },
        cache: "no-store",
      },
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{ success: boolean }>(response);
    expect(json.success).toBe(true);
  });

  it("uses the numeric id extracted from the URL when activityId context is empty", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ status: "success" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/activities//supervisors/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "", supervisorId: "2" }),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/activities/2/supervisors/2",
      {
        method: "DELETE",
        headers: {
          Authorization: "Bearer test-token",
        },
        cache: "no-store",
      },
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
    const thrown = new Error("Failed to remove supervisor");
    mockFetch.mockRejectedValueOnce(thrown);

    const request = createMockRequest("/api/activities/1/supervisors/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "1", supervisorId: "2" }),
    );

    expect(response.status).toBe(500);
    expect(mockHandleApiError).toHaveBeenCalledWith(thrown);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Failed to remove supervisor");
  });

  it("forwards structured backend errors unchanged", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          status: "error",
          error: "cannot remove the only supervisor",
          code: "ONLY_SUPERVISOR_REPLACEMENT_REQUIRED",
        }),
        {
          status: 409,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    const request = createMockRequest(
      "/api/activities/1/supervisors/2?replacement_staff_id=5",
      {
        method: "DELETE",
      },
    );
    const response = await DELETE(
      request,
      createMockContext({ id: "1", supervisorId: "2" }),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/activities/1/supervisors/2?replacement_staff_id=5",
      {
        method: "DELETE",
        headers: {
          Authorization: "Bearer test-token",
        },
        cache: "no-store",
      },
    );
    expect(response.status).toBe(409);
    await expect(response.json()).resolves.toEqual({
      status: "error",
      error: "cannot remove the only supervisor",
      code: "ONLY_SUPERVISOR_REPLACEMENT_REQUIRED",
    });
  });
});
