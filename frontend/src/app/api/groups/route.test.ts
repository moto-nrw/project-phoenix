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

describe("GET /api/groups", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/groups");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches groups from backend API", async () => {
    const mockGroups = [
      { id: 1, name: "Group A" },
      { id: 2, name: "Group B" },
    ];
    mockApiGet.mockResolvedValueOnce({ data: mockGroups });

    const request = createMockRequest("/api/groups");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith("/api/groups", "test-token");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockGroups>>(response);
    expect(json.data).toEqual(mockGroups);
  });

  it("forwards query parameters to backend", async () => {
    mockApiGet.mockResolvedValueOnce({ data: [] });

    const request = createMockRequest("/api/groups?type=ogs&status=active");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/groups?type=ogs&status=active",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("returns empty array when backend returns no data", async () => {
    mockApiGet.mockResolvedValueOnce({ data: null });

    const request = createMockRequest("/api/groups");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });
});

describe("POST /api/groups", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/groups", {
      method: "POST",
      body: { name: "New Group" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("creates a new group via backend API", async () => {
    const createRequest = {
      name: "New Group",
      description: "Test group",
      room_id: 10,
    };
    const mockCreatedGroup = {
      id: 1,
      name: "New Group",
      description: "Test group",
      room_id: 10,
    };
    mockApiPost.mockResolvedValueOnce(mockCreatedGroup);

    const request = createMockRequest("/api/groups", {
      method: "POST",
      body: createRequest,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/groups",
      "test-token",
      createRequest,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockCreatedGroup>>(response);
    expect(json.data).toEqual(mockCreatedGroup);
  });

  it("creates group with minimal required fields", async () => {
    const createRequest = { name: "Minimal Group" };
    const mockCreatedGroup = { id: 2, name: "Minimal Group" };
    mockApiPost.mockResolvedValueOnce(mockCreatedGroup);

    const request = createMockRequest("/api/groups", {
      method: "POST",
      body: createRequest,
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(200);
    const json =
      await parseJsonResponse<ApiResponse<typeof mockCreatedGroup>>(response);
    expect(json.data.name).toBe("Minimal Group");
  });
});
