import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, POST } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiGet, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", () => ({
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
    mockAuth.mockReset().mockResolvedValue(defaultSession);
    mockApiGet.mockReset();
    mockApiPost.mockReset();
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/5/supervisors");
    const response = await GET(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(401);
  });

  it("fetches supervisors for activity", async () => {
    const mockSupervisors = [
      {
        id: 10,
        staff_id: 110,
        first_name: "Mr.",
        last_name: "Smith",
        is_primary: true,
      },
      {
        id: 11,
        staff_id: 111,
        first_name: "Ms.",
        last_name: "Johnson",
        is_primary: false,
      },
    ];

    mockApiGet.mockResolvedValueOnce({ data: mockSupervisors });

    const request = createMockRequest("/api/activities/5/supervisors");
    const response = await GET(request, createMockContext({ id: "5" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/activities/5/supervisors",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{
        id: string;
        staff_id: string;
        name: string;
        is_primary: boolean;
      }>;
    }>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]?.id).toBe("10");
    expect(json.data[0]?.staff_id).toBe("110");
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
    mockAuth.mockReset().mockResolvedValue(defaultSession);
    mockApiGet.mockReset();
    mockApiPost.mockReset();
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
    mockApiPost.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/activities/5/supervisors", {
      method: "POST",
      body: { staff_id: "15", is_primary: true },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/activities/5/supervisors",
      "test-token",
      {
        staff_id: 15,
        is_primary: true,
      },
    );
    expect(response.status).toBe(200);
    expect(mockApiGet).not.toHaveBeenCalled();

    const json = await parseJsonResponse<{
      success: boolean;
    }>(response);
    expect(json.success).toBe(true);
  });

  it("assigns supervisor without is_primary flag", async () => {
    mockApiPost.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/activities/5/supervisors", {
      method: "POST",
      body: { staff_id: "20" },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/activities/5/supervisors",
      "test-token",
      {
        staff_id: 20,
        is_primary: undefined,
      },
    );
    expect(response.status).toBe(200);
    expect(mockApiGet).not.toHaveBeenCalled();
  });

  it("does not refetch after a successful assignment", async () => {
    mockApiPost.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/activities/5/supervisors", {
      method: "POST",
      body: { staff_id: "20" },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(200);
    expect(mockApiGet).not.toHaveBeenCalled();

    const json = await parseJsonResponse<{
      success: boolean;
    }>(response);
    expect(json.success).toBe(true);
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
    mockApiPost.mockRejectedValueOnce(new Error("Failed to assign supervisor"));

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
