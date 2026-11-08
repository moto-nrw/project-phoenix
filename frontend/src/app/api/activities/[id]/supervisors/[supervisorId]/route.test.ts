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

const { mockAuth, mockUncachedAuth, mockFetch, mockGetServerApiUrl } =
  vi.hoisted(() => ({
    mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
    mockUncachedAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
    mockFetch: vi.fn(),
    mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
  }));

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
    uncachedAuth: mockUncachedAuth,
  }),
);

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

// ============================================================================
// Test Helpers
// ============================================================================

function jsonResponse(data: unknown, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

/** The backend call: method, bearer, and Content-Type only with a body. */
function backendInit(method: string, token: string, body?: unknown) {
  return expect.objectContaining({
    method,
    body: body === undefined ? undefined : JSON.stringify(body),
    cache: "no-store",
    signal: expect.any(AbortSignal) as AbortSignal,
    headers: expect.objectContaining({
      Authorization: `Bearer ${token}`,
      ...(body !== undefined && { "Content-Type": "application/json" }),
    }) as Record<string, string>,
  }) as RequestInit;
}

/** A call without body carries the bearer but no Content-Type. */
function expectBearer(call: number, token: string) {
  const init = mockFetch.mock.calls[call]?.[1] as RequestInit | undefined;
  const headers = init?.headers;
  expect(headers).toMatchObject({ Authorization: `Bearer ${token}` });
  expect(headers).not.toHaveProperty("Content-Type");
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
    mockFetch.mockReset();
    mockAuth.mockResolvedValue(defaultSession);
    mockUncachedAuth.mockResolvedValue(defaultSession);
  });

  it("updates supervisor role successfully", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ status: "success" }));

    const request = createMockRequest("/api/activities/1/supervisors/2", {
      method: "PUT",
      body: { is_primary: true },
    });
    const response = await PUT(
      request,
      createMockContext({ id: "1", supervisorId: "2" }),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/activities/1/supervisors/2",
      backendInit("PUT", "test-token", { is_primary: true }),
    );
    expect(response.status).toBe(200);
    // No refetch after the update: the PUT is the only backend call.
    expect(mockFetch).toHaveBeenCalledTimes(1);

    const json = await parseJsonResponse<{ success: boolean }>(response);
    expect(json.success).toBe(true);
  });

  it("does not fail a successful update on refetch errors", async () => {
    mockFetch
      .mockResolvedValueOnce(jsonResponse({ status: "success" }))
      .mockRejectedValueOnce(new Error("Transient list refresh failure"));

    const request = createMockRequest("/api/activities/1/supervisors/2", {
      method: "PUT",
      body: { is_primary: true },
    });
    const response = await PUT(
      request,
      createMockContext({ id: "1", supervisorId: "2" }),
    );

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(1);

    const json = await parseJsonResponse<{ success: boolean }>(response);
    expect(json.success).toBe(true);
  });

  it("uses the numeric id extracted from the URL when activityId context is empty", async () => {
    mockFetch.mockRejectedValueOnce(
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

    // A network failure answers 503 general.unavailable (was 500).
    expect(response.status).toBe(503);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/activities/2/supervisors/2",
      backendInit("PUT", "test-token", { is_primary: true }),
    );
    const json = await parseJsonResponse<{ detail: string; code: string }>(
      response,
    );
    expect(json.code).toBe("general.unavailable");
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

    // A missing route param is the request's mistake: 400 before any call.
    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Invalid supervisorId parameter");
    expect(mockFetch).not.toHaveBeenCalled();
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

    // The route refuses the body itself (bffError 400), before any call.
    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("is_primary parameter is required");
  });
});

describe("DELETE /api/activities/[id]/supervisors/[supervisorId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockReset();
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
      backendInit("DELETE", "test-token"),
    );
    expectBearer(0, "test-token");
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
      backendInit("DELETE", "test-token"),
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

    // A missing route param is the request's mistake: 400 before any call.
    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Invalid supervisorId parameter");
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("returns 503 when the backend is unreachable during removal", async () => {
    const thrown = new Error("Failed to remove supervisor");
    mockFetch.mockRejectedValueOnce(thrown);

    const request = createMockRequest("/api/activities/1/supervisors/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "1", supervisorId: "2" }),
    );

    // A network failure answers 503 general.unavailable (was 500).
    expect(response.status).toBe(503);
    expect(mockFetch).toHaveBeenCalledTimes(1);
    const json = await parseJsonResponse<{ detail: string; code: string }>(
      response,
    );
    expect(json.code).toBe("general.unavailable");
    expect(JSON.stringify(json)).not.toContain(thrown.message);
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
      backendInit("DELETE", "test-token"),
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
      backendInit("DELETE", "test-token"),
    );
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      "http://localhost:8080/api/activities/1/supervisors/2",
      backendInit("DELETE", "fresh-token"),
    );
    expectBearer(0, "test-token");
    expectBearer(1, "fresh-token");
    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual({ success: true });
  });

  it("answers 503 when the renewal keeps the old token (undecided)", async () => {
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
    // A renewal that could not be decided leaves the session: 503.
    expect(response.status).toBe(503);
    await expect(response.json()).resolves.toMatchObject({
      code: "general.unavailable",
    });
  });
});
