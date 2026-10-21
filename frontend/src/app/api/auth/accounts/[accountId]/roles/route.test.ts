import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { GET, PUT } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

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
  return { ...actual, apiGet: mockApiGet, apiPut: mockApiPut };
});

// ============================================================================
// Test Helpers
// ============================================================================

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/auth/accounts/[accountId]/roles", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 500 when accountId parameter is missing", async () => {
    const request = createMockRequest("/api/auth/accounts/undefined/roles");
    const response = await GET(request, createMockContext({}));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Account ID is required");
  });

  it("fetches roles for valid accountId", async () => {
    const mockRoles = [
      {
        id: 1,
        name: "teacher",
        description: "Teacher role",
        created_at: "2024-01-15T10:00:00Z",
      },
      {
        id: 2,
        name: "admin",
        description: "Administrator role",
        created_at: "2024-01-15T10:00:00Z",
      },
    ];
    const mockResponse = {
      status: "success",
      data: mockRoles,
    };
    mockApiGet.mockResolvedValueOnce(mockResponse);

    const request = createMockRequest("/api/auth/accounts/123/roles");
    const response = await GET(
      request,
      createMockContext({ accountId: "123" }),
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/auth/accounts/123/roles",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockResponse>>(response);
    expect(json.data).toEqual(mockResponse);
  });

  it("returns empty array when account has no roles", async () => {
    const mockResponse = {
      status: "success",
      data: [],
    };
    mockApiGet.mockResolvedValueOnce(mockResponse);

    const request = createMockRequest("/api/auth/accounts/123/roles");
    const response = await GET(
      request,
      createMockContext({ accountId: "123" }),
    );

    expect(response.status).toBe(200);
    const json =
      await parseJsonResponse<ApiResponse<typeof mockResponse>>(response);
    expect(json.data).toEqual(mockResponse);
  });
});

describe("PUT /api/auth/accounts/[accountId]/roles", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("forwards the exact role id to the authenticated backend route", async () => {
    mockApiPut.mockResolvedValueOnce(undefined);

    const request = new NextRequest(
      "http://localhost:3000/api/auth/accounts/9007199254740993/roles",
      {
        method: "PUT",
        body: JSON.stringify({ role_id: "9007199254740995" }),
        headers: { "Content-Type": "application/json" },
      },
    );
    const response = await PUT(
      request,
      createMockContext({ accountId: "9007199254740993" }),
    );

    expect(mockApiPut).toHaveBeenCalledWith(
      "/auth/accounts/9007199254740993/roles",
      "test-token",
      { role_id: "9007199254740995" },
    );
    expect(response.status).toBe(200);
  });
});
