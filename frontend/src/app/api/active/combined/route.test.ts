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

describe("GET /api/active/combined", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/combined");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches list of combined groups from backend", async () => {
    const mockCombinedGroups = [
      {
        id: 1,
        name: "Combined Group A",
        description: "Test group",
        room_id: 5,
      },
      {
        id: 2,
        name: "Combined Group B",
        description: "Another group",
        room_id: null,
      },
    ];
    mockApiGet.mockResolvedValueOnce(mockCombinedGroups);

    const request = createMockRequest("/api/active/combined");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/combined",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockCombinedGroups>>(response);
    expect(json.data).toEqual(mockCombinedGroups);
  });

  it("handles empty list", async () => {
    mockApiGet.mockResolvedValueOnce([]);

    const request = createMockRequest("/api/active/combined");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });

  it("handles backend errors gracefully", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Backend error (500)"));

    const request = createMockRequest("/api/active/combined");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
  });
});

describe("POST /api/active/combined", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/combined", {
      method: "POST",
      body: { name: "New Combined Group" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("creates a new combined group", async () => {
    const createRequest = {
      name: "New Combined Group",
      description: "Test description",
      room_id: "5",
    };
    const mockCreatedGroup = {
      id: 10,
      name: "New Combined Group",
      description: "Test description",
      room_id: 5,
    };
    mockApiPost.mockResolvedValueOnce(mockCreatedGroup);

    const request = createMockRequest("/api/active/combined", {
      method: "POST",
      body: createRequest,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/active/combined",
      "test-token",
      createRequest,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockCreatedGroup>>(response);
    expect(json.data).toEqual(mockCreatedGroup);
  });

  it("creates combined group without optional fields", async () => {
    const createRequest = {
      name: "Minimal Group",
    };
    const mockCreatedGroup = {
      id: 11,
      name: "Minimal Group",
      description: null,
      room_id: null,
    };
    mockApiPost.mockResolvedValueOnce(mockCreatedGroup);

    const request = createMockRequest("/api/active/combined", {
      method: "POST",
      body: createRequest,
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockCreatedGroup>>(response);
    expect(json.data.name).toBe("Minimal Group");
  });

  it("handles validation errors", async () => {
    mockApiPost.mockRejectedValueOnce(new Error("Name is required (400)"));

    const request = createMockRequest("/api/active/combined", {
      method: "POST",
      body: { description: "Missing name" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
  });

  it("handles backend errors gracefully", async () => {
    mockApiPost.mockRejectedValueOnce(new Error("Backend error (500)"));

    const request = createMockRequest("/api/active/combined", {
      method: "POST",
      body: { name: "Test Group" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
  });
});
