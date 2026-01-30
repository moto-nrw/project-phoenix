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
  extractParams: vi.fn((request: NextRequest) => {
    const params: Record<string, string> = {};
    request.nextUrl.searchParams.forEach((value: string, key: string) => {
      params[key] = value;
    });
    return params;
  }),
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

describe("GET /api/active/supervisors", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/supervisors");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches supervisors without query params", async () => {
    const mockSupervisors = [
      { id: 1, staff_id: 10, active_group_id: 5 },
      { id: 2, staff_id: 11, active_group_id: 6 },
    ];
    mockApiGet.mockResolvedValueOnce(mockSupervisors);

    const request = createMockRequest("/api/active/supervisors");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/active/supervisors",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockSupervisors>>(response);
    expect(json.data).toEqual(mockSupervisors);
  });

  it("fetches supervisors with query params", async () => {
    const mockSupervisors = [{ id: 1, staff_id: 10, active_group_id: 5 }];
    mockApiGet.mockResolvedValueOnce(mockSupervisors);

    const request = createMockRequest(
      "/api/active/supervisors?staff_id=10&active=true",
    );
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/active/supervisors?staff_id=10&active=true",
      "test-token",
    );
    expect(response.status).toBe(200);
  });
});

describe("POST /api/active/supervisors", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/supervisors", {
      method: "POST",
      body: { staff_id: "10", active_group_id: "5" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("creates a supervisor", async () => {
    const createBody = { staff_id: "10", active_group_id: "5" };
    const mockCreatedSupervisor = { id: 1, ...createBody };
    mockApiPost.mockResolvedValueOnce(mockCreatedSupervisor);

    const request = createMockRequest("/api/active/supervisors", {
      method: "POST",
      body: createBody,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/active/supervisors",
      "test-token",
      createBody,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockCreatedSupervisor>>(
        response,
      );
    expect(json.data).toEqual(mockCreatedSupervisor);
  });
});
