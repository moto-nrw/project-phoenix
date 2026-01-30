import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, PUT, DELETE } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

global.fetch = mockFetch as typeof fetch;

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

function createMockContext(params: Record<string, string> = {}) {
  return { params: Promise.resolve(params) } as {
    params: Promise<{ roleId: string }>;
  };
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

function createMockResponse(data: unknown, status = 200) {
  return Promise.resolve({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(data),
    text: () => Promise.resolve(JSON.stringify(data)),
  } as Response);
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/auth/roles/[roleId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 400 when roleId is missing", async () => {
    const request = createMockRequest("/api/auth/roles/");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(400);
    const json = (await response.json()) as { error: string };
    expect(json.error).toContain("required");
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/roles/1");
    const response = await GET(request, createMockContext({ roleId: "1" }));

    expect(response.status).toBe(401);
  });

  it("fetches role by ID", async () => {
    const mockRole = {
      data: {
        id: 1,
        name: "Admin",
        description: "Administrator role",
        permissions: [{ id: 1, name: "users:read" }],
      },
    };
    mockFetch.mockReturnValueOnce(createMockResponse(mockRole));

    const request = createMockRequest("/api/auth/roles/1");
    const response = await GET(request, createMockContext({ roleId: "1" }));

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/auth/roles/1"),
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as { data: { name: string } };
    expect(json.data.name).toBe("Admin");
  });

  it("handles 404 when role not found", async () => {
    mockFetch.mockReturnValueOnce(
      createMockResponse({ error: "Role not found" }, 404),
    );

    const request = createMockRequest("/api/auth/roles/999");
    const response = await GET(request, createMockContext({ roleId: "999" }));

    expect(response.status).toBe(404);
  });
});

describe("PUT /api/auth/roles/[roleId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 400 when roleId is missing", async () => {
    const request = createMockRequest("/api/auth/roles/", {
      method: "PUT",
      body: { name: "Updated" },
    });
    const response = await PUT(request, createMockContext());

    expect(response.status).toBe(400);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/roles/1", {
      method: "PUT",
      body: { name: "Updated" },
    });
    const response = await PUT(request, createMockContext({ roleId: "1" }));

    expect(response.status).toBe(401);
  });

  it("updates role successfully", async () => {
    mockFetch.mockReturnValueOnce(createMockResponse({ success: true }));

    const request = createMockRequest("/api/auth/roles/1", {
      method: "PUT",
      body: { name: "Updated Admin", description: "Updated description" },
    });
    const response = await PUT(request, createMockContext({ roleId: "1" }));

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/auth/roles/1"),
      expect.objectContaining({
        method: "PUT",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as { success: boolean };
    expect(json.success).toBe(true);
  });
});

describe("DELETE /api/auth/roles/[roleId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 400 when roleId is missing", async () => {
    const request = createMockRequest("/api/auth/roles/", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext());

    expect(response.status).toBe(400);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/roles/1", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ roleId: "1" }));

    expect(response.status).toBe(401);
  });

  it("deletes role successfully", async () => {
    mockFetch.mockReturnValueOnce(createMockResponse({ success: true }));

    const request = createMockRequest("/api/auth/roles/1", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ roleId: "1" }));

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/auth/roles/1"),
      expect.objectContaining({
        method: "DELETE",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as { success: boolean };
    expect(json.success).toBe(true);
  });

  it("handles backend errors during deletion", async () => {
    mockFetch.mockReturnValueOnce(
      createMockResponse({ error: "Cannot delete role in use" }, 409),
    );

    const request = createMockRequest("/api/auth/roles/1", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ roleId: "1" }));

    expect(response.status).toBe(409);
  });
});
