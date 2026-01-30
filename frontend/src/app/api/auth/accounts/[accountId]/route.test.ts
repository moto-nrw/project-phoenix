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

describe("GET /api/auth/accounts/[accountId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/accounts/1");
    const response = await GET(request, createMockContext({ accountId: "1" }));

    expect(response.status).toBe(401);
  });

  it("returns error when accountId is missing", async () => {
    const request = createMockRequest("/api/auth/accounts/");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      success: boolean;
      data: { status: string; message: string };
    };
    // createGetHandler wraps the return value in { success, message, data }
    expect(json.success).toBe(true);
    expect(json.data.status).toBe("error");
    expect(json.data.message).toContain("required");
  });

  it("fetches account from user API successfully", async () => {
    const mockUser = {
      data: {
        account_id: "123",
        email: "teacher@example.com",
        username: "teacher1",
        first_name: "Jane",
      },
    };
    mockFetch.mockReturnValueOnce(createMockResponse(mockUser));

    const request = createMockRequest("/api/auth/accounts/123");
    const response = await GET(
      request,
      createMockContext({ accountId: "123" }),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/users/123"),
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      success: boolean;
      data: {
        status: string;
        data: { id: string; email: string; username: string };
      };
    };
    expect(json.success).toBe(true);
    expect(json.data.status).toBe("success");
    expect(json.data.data).toBeDefined();
    expect(json.data.data.id).toBe("123");
  });

  it("returns fallback account when user API fails", async () => {
    mockFetch.mockReturnValueOnce(
      createMockResponse({ error: "Not found" }, 404),
    );

    const request = createMockRequest("/api/auth/accounts/456");
    const response = await GET(
      request,
      createMockContext({ accountId: "456" }),
    );

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      success: boolean;
      data: {
        status: string;
        data: { id: string; email: string; username: string };
      };
    };
    expect(json.success).toBe(true);
    expect(json.data.status).toBe("success");
    expect(json.data.data.id).toBe("456");
    expect(json.data.data.email).toBe("user@example.com");
    expect(json.data.data.username).toBe("user_456");
  });

  it("handles user data without data wrapper", async () => {
    const mockUser = {
      account_id: "789",
      email: "admin@example.com",
      username: "admin1",
      first_name: "John",
    };
    mockFetch.mockReturnValueOnce(createMockResponse(mockUser));

    const request = createMockRequest("/api/auth/accounts/789");
    const response = await GET(
      request,
      createMockContext({ accountId: "789" }),
    );

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      success: boolean;
      data: { status: string; data: { id: string } };
    };
    expect(json.success).toBe(true);
    expect(json.data.status).toBe("success");
    expect(json.data.data.id).toBe("789");
  });

  it("uses first_name as username fallback", async () => {
    const mockUser = {
      data: {
        account_id: "999",
        email: "test@example.com",
        first_name: "TestUser",
      },
    };
    mockFetch.mockReturnValueOnce(createMockResponse(mockUser));

    const request = createMockRequest("/api/auth/accounts/999");
    const response = await GET(
      request,
      createMockContext({ accountId: "999" }),
    );

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      success: boolean;
      data: { data: { username: string } };
    };
    expect(json.success).toBe(true);
    expect(json.data.data.username).toBe("TestUser");
  });
});
