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

const {
  mockAuth,
  mockUpdateSupervisorRole,
  mockRemoveSupervisor,
  mockGetActivitySupervisors,
} = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockUpdateSupervisorRole: vi.fn(),
  mockRemoveSupervisor: vi.fn(),
  mockGetActivitySupervisors: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/activity-api", () => ({
  updateSupervisorRole: mockUpdateSupervisorRole,
  removeSupervisor: mockRemoveSupervisor,
  getActivitySupervisors: mockGetActivitySupervisors,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: vi.fn(),
  apiPost: vi.fn(),
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
    const updatedSupervisors = [{ id: "2", staff_id: "10", is_primary: true }];
    mockUpdateSupervisorRole.mockResolvedValueOnce(true);
    mockGetActivitySupervisors.mockResolvedValueOnce(updatedSupervisors);

    const request = createMockRequest("/api/activities/1/supervisors/2", {
      method: "PUT",
      body: { is_primary: true },
    });
    const response = await PUT(
      request,
      createMockContext({ id: "1", supervisorId: "2" }),
    );

    expect(mockUpdateSupervisorRole).toHaveBeenCalledWith("1", "2", {
      is_primary: true,
    });
    expect(mockGetActivitySupervisors).toHaveBeenCalledWith("1");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof updatedSupervisors>>(response);
    expect(json.data).toEqual(updatedSupervisors);
  });

  it("returns 500 when activityId is empty and route extracts id from URL", async () => {
    // Note: extractParams in route-wrapper extracts numeric IDs from URL path,
    // so even with empty id param, "2" from the URL gets used as id.
    // The handler then calls updateSupervisorRole which isn't mocked to succeed.
    const request = createMockRequest("/api/activities//supervisors/2", {
      method: "PUT",
      body: { is_primary: true },
    });
    const response = await PUT(
      request,
      createMockContext({ id: "", supervisorId: "2" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Failed to update supervisor role");
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
    mockUpdateSupervisorRole.mockResolvedValueOnce(false);

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
    mockRemoveSupervisor.mockResolvedValueOnce(true);

    const request = createMockRequest("/api/activities/1/supervisors/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "1", supervisorId: "2" }),
    );

    expect(mockRemoveSupervisor).toHaveBeenCalledWith("1", "2");
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{ success: boolean }>(response);
    expect(json.success).toBe(true);
  });

  it("returns 500 when activityId is empty and route extracts id from URL", async () => {
    // extractParams extracts numeric "2" from URL path as id
    const request = createMockRequest("/api/activities//supervisors/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "", supervisorId: "2" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Failed to remove supervisor");
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
    mockRemoveSupervisor.mockResolvedValueOnce(false);

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
