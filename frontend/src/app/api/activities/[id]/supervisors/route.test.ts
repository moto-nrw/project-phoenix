import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, POST } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockGetActivitySupervisors, mockAssignSupervisor } =
  vi.hoisted(() => ({
    mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
    mockGetActivitySupervisors: vi.fn(),
    mockAssignSupervisor: vi.fn(),
  }));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
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

vi.mock("~/lib/activity-api", () => ({
  getActivitySupervisors: mockGetActivitySupervisors,
  assignSupervisor: mockAssignSupervisor,
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

describe("GET /api/activities/[id]/supervisors", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/5/supervisors");
    const response = await GET(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(401);
  });

  it("fetches supervisors for activity", async () => {
    const mockSupervisors = [
      { id: "10", name: "Mr. Smith", is_primary: true },
      { id: "11", name: "Ms. Johnson", is_primary: false },
    ];

    mockGetActivitySupervisors.mockResolvedValueOnce(mockSupervisors);

    const request = createMockRequest("/api/activities/5/supervisors");
    const response = await GET(request, createMockContext({ id: "5" }));

    expect(mockGetActivitySupervisors).toHaveBeenCalledWith("5");
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string; name: string; is_primary: boolean }>;
    }>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]?.name).toBe("Mr. Smith");
    expect(json.data[0]?.is_primary).toBe(true);
  });

  it("throws error when activity ID is missing", async () => {
    const request = createMockRequest("/api/activities//supervisors");
    const response = await GET(request, createMockContext({ id: "" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Activity ID is required");
  });
});

describe("POST /api/activities/[id]/supervisors", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/5/supervisors", {
      method: "POST",
      body: { staff_id: "15", is_primary: true },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(401);
  });

  it("assigns supervisor successfully", async () => {
    mockAssignSupervisor.mockResolvedValueOnce(true);

    const mockUpdatedSupervisors = [
      { id: "15", name: "Mr. Brown", is_primary: true },
    ];
    mockGetActivitySupervisors.mockResolvedValueOnce(mockUpdatedSupervisors);

    const request = createMockRequest("/api/activities/5/supervisors", {
      method: "POST",
      body: { staff_id: "15", is_primary: true },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(mockAssignSupervisor).toHaveBeenCalledWith("5", {
      staff_id: "15",
      is_primary: true,
    });
    expect(mockGetActivitySupervisors).toHaveBeenCalledWith("5");
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string; name: string }>;
    }>(response);
    expect(json.data).toHaveLength(1);
    expect(json.data[0]?.name).toBe("Mr. Brown");
  });

  it("assigns supervisor without is_primary flag", async () => {
    mockAssignSupervisor.mockResolvedValueOnce(true);
    mockGetActivitySupervisors.mockResolvedValueOnce([]);

    const request = createMockRequest("/api/activities/5/supervisors", {
      method: "POST",
      body: { staff_id: "20" },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(mockAssignSupervisor).toHaveBeenCalledWith("5", {
      staff_id: "20",
      is_primary: undefined,
    });
    expect(response.status).toBe(200);
  });

  it("throws error when activity ID is missing", async () => {
    const request = createMockRequest("/api/activities//supervisors", {
      method: "POST",
      body: { staff_id: "15" },
    });
    const response = await POST(request, createMockContext({ id: "" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Activity ID is required");
  });

  it("throws error when staff_id is missing", async () => {
    const request = createMockRequest("/api/activities/5/supervisors", {
      method: "POST",
      body: {},
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Staff ID is required");
  });

  it("throws error when assignment fails", async () => {
    mockAssignSupervisor.mockResolvedValueOnce(false);

    const request = createMockRequest("/api/activities/5/supervisors", {
      method: "POST",
      body: { staff_id: "15" },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Failed to assign supervisor");
  });
});
