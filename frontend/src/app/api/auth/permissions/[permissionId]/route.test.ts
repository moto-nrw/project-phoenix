import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET, PUT, DELETE } from "./route";
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

function createMockContext(params: Record<string, string> = {}) {
  return { params: Promise.resolve(params) } as {
    params: Promise<{ permissionId: string }>;
  };
}

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

describe("GET /api/auth/permissions/[permissionId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 400 when permissionId is missing", async () => {
    const request = createMockRequest("/api/auth/permissions/");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(400);
    const json = (await response.json()) as { detail: string };
    expect(json.detail).toContain("required");
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/permissions/1");
    const response = await GET(
      request,
      createMockContext({ permissionId: "1" }),
    );

    expect(response.status).toBe(401);
  });

  it("fetches permission by ID", async () => {
    const mockPermission = {
      data: {
        id: 1,
        name: "users:read",
        resource: "users",
        action: "read",
        description: "Read users",
      },
    };
    mockFetch.mockReturnValueOnce(createMockResponse(mockPermission));

    const request = createMockRequest("/api/auth/permissions/1");
    const response = await GET(
      request,
      createMockContext({ permissionId: "1" }),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/auth/permissions/1"),
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as { data: { name: string } };
    expect(json.data.name).toBe("users:read");
  });

  it("handles 404 when permission not found", async () => {
    mockFetch.mockReturnValueOnce(
      createMockResponse({ detail: "Permission not found" }, 404),
    );

    const request = createMockRequest("/api/auth/permissions/999");
    const response = await GET(
      request,
      createMockContext({ permissionId: "999" }),
    );

    expect(response.status).toBe(404);
  });
});

describe("PUT /api/auth/permissions/[permissionId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 400 when permissionId is missing", async () => {
    const request = createMockRequest("/api/auth/permissions/", {
      method: "PUT",
      body: { description: "Updated" },
    });
    const response = await PUT(request, createMockContext());

    expect(response.status).toBe(400);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/permissions/1", {
      method: "PUT",
      body: { description: "Updated" },
    });
    const response = await PUT(
      request,
      createMockContext({ permissionId: "1" }),
    );

    expect(response.status).toBe(401);
  });

  it("updates permission successfully", async () => {
    const updatedPermission = {
      data: {
        id: 1,
        name: "users:read",
        description: "Updated description",
      },
    };
    mockFetch.mockReturnValueOnce(createMockResponse(updatedPermission));

    const request = createMockRequest("/api/auth/permissions/1", {
      method: "PUT",
      body: { description: "Updated description" },
    });
    const response = await PUT(
      request,
      createMockContext({ permissionId: "1" }),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/auth/permissions/1"),
      expect.objectContaining({
        method: "PUT",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as { data: { description: string } };
    expect(json.data.description).toBe("Updated description");
  });
});

describe("DELETE /api/auth/permissions/[permissionId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 400 when permissionId is missing", async () => {
    const request = createMockRequest("/api/auth/permissions/", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext());

    expect(response.status).toBe(400);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/permissions/1", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ permissionId: "1" }),
    );

    expect(response.status).toBe(401);
  });

  it("deletes permission successfully", async () => {
    mockFetch.mockReturnValueOnce(createMockResponse(null, 204));

    const request = createMockRequest("/api/auth/permissions/1", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ permissionId: "1" }),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/auth/permissions/1"),
      expect.objectContaining({
        method: "DELETE",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );
    expect(response.status).toBe(204);
  });

  it("handles backend errors during deletion", async () => {
    mockFetch.mockReturnValueOnce(
      createMockResponse({ detail: "Cannot delete permission in use" }, 409),
    );

    const request = createMockRequest("/api/auth/permissions/1", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ permissionId: "1" }),
    );

    expect(response.status).toBe(409);
  });
});
