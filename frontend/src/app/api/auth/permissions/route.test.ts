import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET, POST } from "./route";
import {
  type ExtendedSession,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

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

function createMockResponse(data: unknown, status = 200) {
  return Promise.resolve(
    status === 204
      ? new Response(null, { status })
      : Response.json(data, { status }),
  );
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/auth/permissions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
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
});

describe("POST /api/auth/permissions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
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
});
