import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { GET, POST } from "./route";
import { ApiResponseError } from "~/lib/api-helpers.server";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

const { captureException } = vi.hoisted(() => ({ captureException: vi.fn() }));
vi.mock("@sentry/nextjs", () => ({ captureException }));

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

vi.mock("@/lib/api-helpers.server", async (importOriginal) => {
  const actual = await importOriginal<Record<string, unknown>>();
  return {
    ...actual,
    apiGet: mockApiGet,
    apiPut: mockApiPut,
  };
});

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/auth/accounts", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
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
    // Handler returns the full apiGet result ({ data: [...] }), wrapper wraps it in { success, data }
    expect((json.data as unknown as { data: unknown[] }).data).toHaveLength(2);
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
});

describe("POST /api/auth/accounts", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
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

    expect(mockApiPut).toHaveBeenCalledWith("/auth/accounts/1", "test-token", {
      active: false,
    });
    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data: {
        id: number;
        email: string;
        active: boolean;
      };
    };
    // POST handler returns Response.json(response) where response is the full apiPut result
    // apiPut returns { data: { id: 1, ... } }, so json = { data: { id: 1, ... } }
    expect(json.data.active).toBe(false);
  });

  it("handles errors during update", async () => {
    const failure = new Error("Update failed");
    mockApiPut.mockRejectedValueOnce(failure);

    const request = createMockRequest("/api/auth/accounts", {
      method: "POST",
      body: { id: "1", active: false },
    });
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = (await response.json()) as { detail: string };
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Failed to update account");
    expect(captureException).toHaveBeenCalledExactlyOnceWith(failure, {});
  });

  it("forwards a backend conflict body and status unchanged", async () => {
    const body = JSON.stringify({
      type: "about:blank",
      title: "Conflict",
      status: 409,
      detail: "account already exists",
      code: "general.business_rejection",
      details: { account_id: "1" },
      errors: [{ field: "email", reason: "already_used" }],
      instance: "/auth/accounts/1",
    });
    mockApiPut.mockRejectedValueOnce(
      new ApiResponseError(409, body, {
        contentType: "application/problem+json",
      }),
    );

    const response = await POST(
      createMockRequest("/api/auth/accounts", {
        method: "POST",
        body: { id: "1", email: "taken@example.com" },
      }),
    );

    expect(response.status).toBe(409);
    expect(response.headers.get("content-type")).toBe(
      "application/problem+json",
    );
    expect(await response.text()).toBe(body);
    expect(captureException).not.toHaveBeenCalled();
  });

  it("rejects malformed account JSON without an event", async () => {
    const response = await POST(
      new NextRequest("http://localhost:3000/api/auth/accounts", {
        method: "POST",
        body: '{"password":"cleartext-secret"',
      }),
    );

    expect(response.status).toBe(400);
    expect(await response.json()).toMatchObject({
      status: 400,
      detail: "Invalid JSON",
      code: "general.input",
    });
    expect(mockApiPut).not.toHaveBeenCalled();
    expect(captureException).not.toHaveBeenCalled();
  });
});
