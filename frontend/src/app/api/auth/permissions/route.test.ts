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
  options: {
    method?: string;
    body?: unknown;
    searchParams?: Record<string, string>;
  } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");

  if (options.searchParams) {
    Object.entries(options.searchParams).forEach(([key, value]) => {
      url.searchParams.append(key, value);
    });
  }

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

describe("GET /api/auth/permissions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/permissions");
    const response = await GET(request);

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error: string };
    expect(json.error).toBe("Unauthorized");
  });

  it("fetches all permissions", async () => {
    const mockPermissions = {
      data: [
        { id: 1, name: "users:read", resource: "users", action: "read" },
        { id: 2, name: "users:write", resource: "users", action: "write" },
      ],
    };
    mockFetch.mockReturnValueOnce(createMockResponse(mockPermissions));

    const request = createMockRequest("/api/auth/permissions");
    const response = await GET(request);

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/auth/permissions"),
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as { data: unknown[] };
    expect(json.data).toHaveLength(2);
  });

  it("filters permissions by resource", async () => {
    mockFetch.mockReturnValueOnce(createMockResponse({ data: [] }));

    const request = createMockRequest("/api/auth/permissions", {
      searchParams: { resource: "groups" },
    });
    await GET(request);

    const fetchUrl = (mockFetch.mock.calls[0]?.[0] as string) ?? "";
    expect(fetchUrl).toContain("resource=groups");
  });

  it("filters permissions by action", async () => {
    mockFetch.mockReturnValueOnce(createMockResponse({ data: [] }));

    const request = createMockRequest("/api/auth/permissions", {
      searchParams: { action: "delete" },
    });
    await GET(request);

    const fetchUrl = (mockFetch.mock.calls[0]?.[0] as string) ?? "";
    expect(fetchUrl).toContain("action=delete");
  });

  it("handles backend errors", async () => {
    mockFetch.mockReturnValueOnce(
      createMockResponse({ error: "Backend error" }, 500),
    );

    const request = createMockRequest("/api/auth/permissions");
    const response = await GET(request);

    expect(response.status).toBe(500);
  });
});

describe("POST /api/auth/permissions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/permissions", {
      method: "POST",
      body: {
        name: "custom:permission",
        resource: "custom",
        action: "permission",
      },
    });
    const response = await POST(request);

    expect(response.status).toBe(401);
  });

  it("creates a new permission", async () => {
    const newPermission = {
      data: {
        id: 3,
        name: "rooms:manage",
        resource: "rooms",
        action: "manage",
        description: "Manage rooms",
      },
    };
    mockFetch.mockReturnValueOnce(createMockResponse(newPermission));

    const request = createMockRequest("/api/auth/permissions", {
      method: "POST",
      body: {
        name: "rooms:manage",
        resource: "rooms",
        action: "manage",
        description: "Manage rooms",
      },
    });
    const response = await POST(request);

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/auth/permissions"),
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
        body: expect.stringContaining("rooms:manage") as string,
      }),
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as { data: { name: string } };
    expect(json.data.name).toBe("rooms:manage");
  });

  it("handles backend errors during creation", async () => {
    mockFetch.mockReturnValueOnce(
      createMockResponse({ error: "Permission already exists" }, 409),
    );

    const request = createMockRequest("/api/auth/permissions", {
      method: "POST",
      body: { name: "duplicate:permission" },
    });
    const response = await POST(request);

    expect(response.status).toBe(409);
  });
});
