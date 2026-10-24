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
    params: Promise<{ roleId: string }>;
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

describe("GET /api/auth/roles/[roleId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 400 when roleId is missing", async () => {
    const request = createMockRequest("/api/auth/roles/");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(400);
    const json = (await response.json()) as { detail: string };
    expect(json.detail).toContain("required");
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
});
