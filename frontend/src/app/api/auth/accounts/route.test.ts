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

const { mockAuth, mockApiGet, mockApiPut } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPut: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("@/lib/api-client", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: mockApiPut,
  apiDelete: vi.fn(),
}));

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

function createMockContext() {
  return {
    params: Promise.resolve({}),
  };
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/auth/accounts", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/accounts");
    const context = createMockContext();
    const response = await GET(request, context);

    expect(response.status).toBe(401);
  });

  it("fetches all accounts", async () => {
    const mockAccounts = {
      data: [
        { id: 1, email: "admin@example.com", active: true },
        { id: 2, email: "teacher@example.com", active: true },
      ],
    };
    mockApiGet.mockResolvedValueOnce(mockAccounts);

    const request = createMockRequest("/api/auth/accounts");
    const context = createMockContext();
    const response = await GET(request, context);

    expect(mockApiGet).toHaveBeenCalledWith("/auth/accounts", "test-token");
    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      success: boolean;
      data: unknown[];
    };
    // GET handler wraps response in { success: true, message: "Success", data: <handler_return> }
    // Handler returns response.data which is the accounts array
    // So final response is { success: true, message: "Success", data: [...] }
    expect(json.success).toBe(true);
    expect(json.data).toHaveLength(2);
  });

  it("filters accounts by email", async () => {
    mockApiGet.mockResolvedValueOnce({ data: [] });

    const request = createMockRequest("/api/auth/accounts", {
      searchParams: { email: "test@example.com" },
    });
    const context = createMockContext();
    await GET(request, context);

    expect(mockApiGet).toHaveBeenCalledWith(
      "/auth/accounts?email=test%40example.com",
      "test-token",
    );
  });

  it("filters accounts by active status", async () => {
    mockApiGet.mockResolvedValueOnce({ data: [] });

    const request = createMockRequest("/api/auth/accounts", {
      searchParams: { active: "true" },
    });
    const context = createMockContext();
    await GET(request, context);

    expect(mockApiGet).toHaveBeenCalledWith(
      "/auth/accounts?active=true",
      "test-token",
    );
  });

  it("handles backend errors", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Backend error (500)"));

    const request = createMockRequest("/api/auth/accounts");
    const context = createMockContext();
    const response = await GET(request, context);

    expect(response.status).toBe(500);
  });
});

describe("POST /api/auth/accounts", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/accounts", {
      method: "POST",
      body: { id: "1", active: false },
    });
    const response = await POST(request);

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error: string };
    expect(json.error).toBe("Unauthorized");
  });

  it("updates an account via PUT", async () => {
    const updatedAccount = {
      data: {
        id: 1,
        email: "admin@example.com",
        active: false,
      },
    };
    mockApiPut.mockResolvedValueOnce(updatedAccount);

    const request = createMockRequest("/api/auth/accounts", {
      method: "POST",
      body: { id: "1", active: false },
    });
    const response = await POST(request);

    expect(mockApiPut).toHaveBeenCalledWith(
      "/auth/accounts/1",
      { active: false },
      "test-token",
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      id: number;
      email: string;
      active: boolean;
    };
    // POST handler returns Response.json(response.data) without wrapping
    // apiPut returns { data: { id: 1, ... } }, route returns response.data
    // So the response is { id: 1, email: ..., active: false }
    expect(json.active).toBe(false);
  });

  it("handles errors during update", async () => {
    mockApiPut.mockRejectedValueOnce(new Error("Update failed"));

    const request = createMockRequest("/api/auth/accounts", {
      method: "POST",
      body: { id: "1", active: false },
    });
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = (await response.json()) as { error: string };
    expect(json.error).toBe("Failed to update account");
  });
});
