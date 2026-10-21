import { describe, it, expect, vi, beforeEach } from "vitest";
import { PUT, DELETE } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks
// ============================================================================

const {
  mockAuth,
  mockUncachedAuth,
  mockApiPut,
  mockApiGet,
  mockFetch,
  mockGetServerApiUrl,
  mockHandleApiError,
} = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockUncachedAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPut: vi.fn(),
  mockApiGet: vi.fn(),
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
  mockHandleApiError: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockUncachedAuth,
}));

vi.mock("~/lib/api-helpers.server", async () => {
  const helper = await import("~/test/helpers/handle-api-error");
  mockHandleApiError.mockImplementation(helper.mockHandleApiError);
  return {
    apiGet: mockApiGet,
    apiPost: vi.fn(),
    apiPut: mockApiPut,
    apiDelete: vi.fn(),
    handleApiError: mockHandleApiError,
  };
});

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

// ============================================================================
// Test Helpers
// ============================================================================

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
    mockUncachedAuth.mockResolvedValue(defaultSession);
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
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain(
      "Fallback activity id was extracted from URL",
    );
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
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Supervisor ID is required");
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
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain(
      "is_primary parameter is required",
    );
  });
});

describe("DELETE /api/activities/[id]/supervisors/[supervisorId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
    mockUncachedAuth.mockResolvedValue(defaultSession);
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
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Supervisor ID is required");
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
    expect(mockHandleApiError).toHaveBeenCalledWith(thrown, request);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Failed to remove supervisor");
  });

  it("forwards structured backend errors unchanged", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          detail: "cannot remove the only supervisor",
          code: "timetable.only_supervisor_replacement_required",
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
      detail: "cannot remove the only supervisor",
      code: "timetable.only_supervisor_replacement_required",
    });
  });

  it("retries once with uncachedAuth when the backend returns 401", async () => {
    mockUncachedAuth.mockResolvedValueOnce({
      ...defaultSession,
      user: { ...defaultSession.user, token: "fresh-token" },
    });
    mockFetch
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ detail: "expired" }), {
          status: 401,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
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

    expect(mockFetch).toHaveBeenNthCalledWith(
      1,
      "http://localhost:8080/api/activities/1/supervisors/2",
      {
        method: "DELETE",
        headers: {
          Authorization: "Bearer test-token",
        },
        cache: "no-store",
      },
    );
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      "http://localhost:8080/api/activities/1/supervisors/2",
      {
        method: "DELETE",
        headers: {
          Authorization: "Bearer fresh-token",
        },
        cache: "no-store",
      },
    );
    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual({ success: true });
  });

  it("answers token expired when refresh does not yield a new token", async () => {
    mockUncachedAuth.mockResolvedValueOnce(defaultSession);
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ detail: "expired" }), {
        status: 401,
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

    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(response.status).toBe(401);
    await expect(response.json()).resolves.toMatchObject({
      status: 401,
      detail: "Token expired",
      code: "general.permission",
    });
  });
});
