import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET, PUT, DELETE } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockFetch: vi.fn<typeof fetch>(),
}));

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
    uncachedAuth: mockAuth,
  }),
);

vi.stubGlobal("fetch", mockFetch);

function backendJson(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// ============================================================================
// Test Helpers
// ============================================================================

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

describe("GET /api/active/supervisors/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("fetches supervisor by ID", async () => {
    const mockSupervisor = { id: 1, staff_id: 10, active_group_id: 5 };
    mockFetch.mockResolvedValueOnce(backendJson(mockSupervisor));

    const request = createMockRequest("/api/active/supervisors/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/api/active/supervisors/1",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }),
      }),
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockSupervisor>>(response);
    expect(json.data).toEqual(mockSupervisor);
  });

  it("returns error when id is invalid", async () => {
    const request = createMockRequest("/api/active/supervisors/invalid");
    const response = await GET(request, createMockContext({ id: undefined }));

    // A missing or invalid route param is the request's mistake: 400.
    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Invalid id parameter");
  });
});

describe("PUT /api/active/supervisors/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("updates supervisor via backend", async () => {
    const updateBody = { active_group_id: "6" };
    const mockUpdatedSupervisor = {
      id: 1,
      staff_id: 10,
      active_group_id: 6,
    };
    mockFetch.mockResolvedValueOnce(backendJson(mockUpdatedSupervisor));

    const request = createMockRequest("/api/active/supervisors/1", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/api/active/supervisors/1",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify(updateBody),
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
          "Content-Type": "application/json",
        }),
      }),
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedSupervisor>>(
        response,
      );
    expect(json.data).toEqual(mockUpdatedSupervisor);
  });

  it("returns error when id is invalid", async () => {
    const request = createMockRequest("/api/active/supervisors/invalid", {
      method: "PUT",
      body: { active_group_id: "6" },
    });
    const response = await PUT(request, createMockContext({ id: undefined }));

    // A missing or invalid route param is the request's mistake: 400.
    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Invalid id parameter");
  });
});

describe("DELETE /api/active/supervisors/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("deletes supervisor via backend and returns 204", async () => {
    mockFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const request = createMockRequest("/api/active/supervisors/1", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/api/active/supervisors/1",
      expect.objectContaining({
        method: "DELETE",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }),
      }),
    );
    expect(response.status).toBe(204);
  });

  it("returns error when id is invalid", async () => {
    const request = createMockRequest("/api/active/supervisors/invalid", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: undefined }),
    );

    // A missing or invalid route param is the request's mistake: 400.
    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Invalid id parameter");
  });
});
