import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { POST, DELETE } from "./route";

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
  options: { method?: string } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  return new NextRequest(url, { method: options.method ?? "GET" });
}

function createMockContext(params: Record<string, string> = {}) {
  return { params: Promise.resolve(params) } as {
    params: Promise<{ roleId: string; permissionId: string }>;
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

describe("POST /api/auth/roles/[roleId]/permissions/[permissionId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 400 when roleId or permissionId is missing", async () => {
    const request = createMockRequest("/api/auth/roles//permissions/", {
      method: "POST",
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(400);
    const json = (await response.json()) as { error: string };
    expect(json.error).toContain("required");
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/roles/1/permissions/2", {
      method: "POST",
    });
    const response = await POST(
      request,
      createMockContext({ roleId: "1", permissionId: "2" }),
    );

    expect(response.status).toBe(401);
  });

  it("assigns permission to role successfully", async () => {
    mockFetch.mockReturnValueOnce(createMockResponse({ success: true }));

    const request = createMockRequest("/api/auth/roles/1/permissions/2", {
      method: "POST",
    });
    const response = await POST(
      request,
      createMockContext({ roleId: "1", permissionId: "2" }),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/auth/roles/1/permissions/2"),
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as { success: boolean };
    expect(json.success).toBe(true);
  });

  it("handles backend errors during assignment", async () => {
    mockFetch.mockReturnValueOnce(
      createMockResponse({ error: "Permission already assigned" }, 409),
    );

    const request = createMockRequest("/api/auth/roles/1/permissions/2", {
      method: "POST",
    });
    const response = await POST(
      request,
      createMockContext({ roleId: "1", permissionId: "2" }),
    );

    expect(response.status).toBe(409);
  });
});

describe("DELETE /api/auth/roles/[roleId]/permissions/[permissionId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 400 when roleId or permissionId is missing", async () => {
    const request = createMockRequest("/api/auth/roles//permissions/", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext());

    expect(response.status).toBe(400);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/roles/1/permissions/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ roleId: "1", permissionId: "2" }),
    );

    expect(response.status).toBe(401);
  });

  it("removes permission from role successfully", async () => {
    mockFetch.mockReturnValueOnce(createMockResponse({ success: true }));

    const request = createMockRequest("/api/auth/roles/1/permissions/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ roleId: "1", permissionId: "2" }),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/auth/roles/1/permissions/2"),
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

  it("handles backend errors during removal", async () => {
    mockFetch.mockReturnValueOnce(
      createMockResponse({ error: "Permission not found" }, 404),
    );

    const request = createMockRequest("/api/auth/roles/1/permissions/2", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ roleId: "1", permissionId: "2" }),
    );

    expect(response.status).toBe(404);
  });
});
