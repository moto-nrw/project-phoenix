import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("@/lib/api-client", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
}));

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(path: string): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  return new NextRequest(url);
}

function createMockContext(params: Record<string, string> = {}) {
  return { params: Promise.resolve(params) };
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/auth/roles/[roleId]/permissions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/roles/1/permissions");
    const response = await GET(request, createMockContext({ roleId: "1" }));

    expect(response.status).toBe(401);
  });

  it("fetches permissions for a role", async () => {
    const mockPermissions = {
      data: [
        { id: 1, name: "users:read", resource: "users", action: "read" },
        { id: 2, name: "users:write", resource: "users", action: "write" },
      ],
    };
    mockApiGet.mockResolvedValueOnce(mockPermissions);

    const request = createMockRequest("/api/auth/roles/1/permissions");
    const response = await GET(request, createMockContext({ roleId: "1" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/auth/roles/1/permissions",
      "test-token",
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as { data: unknown[] };
    expect(json.data).toHaveLength(2);
  });

  it("handles backend errors", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Backend error (500)"));

    const request = createMockRequest("/api/auth/roles/1/permissions");
    const response = await GET(request, createMockContext({ roleId: "1" }));

    expect(response.status).toBe(500);
  });
});
