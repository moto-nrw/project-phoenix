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

describe("GET /api/active/groups", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/groups");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches active groups from backend", async () => {
    const mockGroups = [
      { id: 1, name: "OGS Group A", room_id: 10 },
      { id: 2, name: "OGS Group B", room_id: 11 },
    ];
    mockApiGet.mockResolvedValueOnce({ data: mockGroups });

    const request = createMockRequest("/api/active/groups");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith("/api/active/groups", "test-token");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<{ data: typeof mockGroups }>>(
        response,
      );
    expect(json.data.data).toEqual(mockGroups);
  });

  it("supports query parameters", async () => {
    const mockGroups = [{ id: 1, name: "Active Group", room_id: 5 }];
    mockApiGet.mockResolvedValueOnce({ data: mockGroups });

    const request = createMockRequest("/api/active/groups?room_id=5");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/groups?room_id=5",
      "test-token",
    );
    expect(response.status).toBe(200);
  });
});

describe("POST /api/active/groups", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/groups", {
      method: "POST",
      body: { name: "New Group" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("creates a new active group", async () => {
    const createRequest = {
      name: "New OGS Group",
      description: "Test group",
      room_id: "5",
    };
    const mockCreatedGroup = {
      id: 99,
      name: "New OGS Group",
      description: "Test group",
      room_id: 5,
    };
    mockApiPost.mockResolvedValueOnce(mockCreatedGroup);

    const request = createMockRequest("/api/active/groups", {
      method: "POST",
      body: createRequest,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/active/groups",
      "test-token",
      createRequest,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockCreatedGroup>>(response);
    expect(json.data).toEqual(mockCreatedGroup);
  });

  it("creates group with minimal fields", async () => {
    const createRequest = { name: "Minimal Group" };
    const mockCreatedGroup = { id: 88, name: "Minimal Group" };
    mockApiPost.mockResolvedValueOnce(mockCreatedGroup);

    const request = createMockRequest("/api/active/groups", {
      method: "POST",
      body: createRequest,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/active/groups",
      "test-token",
      createRequest,
    );
    expect(response.status).toBe(200);
  });
});
