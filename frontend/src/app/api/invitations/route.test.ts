import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks
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
    // Extract status code from error message like "Error message (409)"
    const statusMatch = /\((\d+)\)/.exec(message);
    const status = statusMatch ? parseInt(statusMatch[1]!, 10) : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
}));

const { GET, POST } = await import("./route");

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

describe("GET /api/invitations", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/invitations");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches invitations list from backend", async () => {
    const invitations = [
      {
        id: 1,
        email: "teacher@example.com",
        role_id: 2,
        token: "inv-token-1",
        expires_at: "2024-02-01T00:00:00Z",
        created_by: 1,
      },
      {
        id: 2,
        email: "staff@example.com",
        role_id: 3,
        token: "inv-token-2",
        expires_at: "2024-02-01T00:00:00Z",
        created_by: 1,
      },
    ];

    mockApiGet.mockResolvedValueOnce({ data: invitations });

    const request = createMockRequest("/api/invitations");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith("/auth/invitations", "test-token");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof invitations>>(response);
    expect(json.data).toEqual(invitations);
  });
});

describe("POST /api/invitations", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/invitations", {
      method: "POST",
      body: { email: "new@example.com", roleId: 2 },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("creates invitation successfully with roleId (camelCase)", async () => {
    const requestBody = {
      email: "newteacher@example.com",
      roleId: 2,
      firstName: "John",
      lastName: "Doe",
      position: "Math Teacher",
    };

    const createdInvitation = {
      id: 10,
      email: "newteacher@example.com",
      role_id: 2,
      token: "new-inv-token",
      expires_at: "2024-02-01T00:00:00Z",
      created_by: 1,
      first_name: "John",
      last_name: "Doe",
      position: "Math Teacher",
    };

    mockApiPost.mockResolvedValueOnce({ data: createdInvitation });

    const request = createMockRequest("/api/invitations", {
      method: "POST",
      body: requestBody,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/auth/invitations",
      "test-token",
      {
        email: "newteacher@example.com",
        role_id: 2,
        first_name: "John",
        last_name: "Doe",
        position: "Math Teacher",
      },
    );

    expect(response.status).toBe(200);
    const json =
      await parseJsonResponse<ApiResponse<typeof createdInvitation>>(response);
    expect(json.data.email).toBe("newteacher@example.com");
  });

  it("creates invitation successfully with role_id (snake_case)", async () => {
    const requestBody = {
      email: "newteacher@example.com",
      role_id: 2,
      first_name: "Jane",
      last_name: "Smith",
    };

    const createdInvitation = {
      id: 11,
      email: "newteacher@example.com",
      role_id: 2,
      token: "new-inv-token-2",
      expires_at: "2024-02-01T00:00:00Z",
      created_by: 1,
      first_name: "Jane",
      last_name: "Smith",
    };

    mockApiPost.mockResolvedValueOnce({ data: createdInvitation });

    const request = createMockRequest("/api/invitations", {
      method: "POST",
      body: requestBody,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/auth/invitations",
      "test-token",
      {
        email: "newteacher@example.com",
        role_id: 2,
        first_name: "Jane",
        last_name: "Smith",
      },
    );

    expect(response.status).toBe(200);
  });

  it("returns error when role_id is missing", async () => {
    const requestBody = {
      email: "newteacher@example.com",
    };

    const request = createMockRequest("/api/invitations", {
      method: "POST",
      body: requestBody,
    });

    // createPostHandler catches errors and returns error response
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Invalid invitation payload: role id missing");
  });

  it("handles backend error", async () => {
    const requestBody = {
      email: "duplicate@example.com",
      roleId: 2,
    };

    mockApiPost.mockRejectedValueOnce(
      new Error("User already has invitation (409)"),
    );

    const request = createMockRequest("/api/invitations", {
      method: "POST",
      body: requestBody,
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(409);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("User already has invitation");
  });
});
