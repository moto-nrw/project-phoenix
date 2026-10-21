import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { PUT } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockApiPut } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPut: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("@/lib/api-helpers.server", async (importOriginal) => {
  const actual = await importOriginal<Record<string, unknown>>();
  return { ...actual, apiPut: mockApiPut };
});

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(
  path: string,
  options: { method?: string; body?: unknown } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const requestInit: { method: string; body?: string; headers?: HeadersInit } =
    {
      method: options.method ?? "PUT",
    };

  if (options.body) {
    requestInit.body = JSON.stringify(options.body);
    requestInit.headers = { "Content-Type": "application/json" };
  }

  return new NextRequest(url, requestInit);
}

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

describe("PUT /api/auth/accounts/[accountId]/deactivate", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("deactivates account successfully", async () => {
    const mockResponse = { message: "Account deactivated" };
    mockApiPut.mockResolvedValueOnce(mockResponse);

    const request = createMockRequest("/api/auth/accounts/123/deactivate", {
      method: "PUT",
    });
    const response = await PUT(
      request,
      createMockContext({ accountId: "123" }),
    );

    expect(mockApiPut).toHaveBeenCalledWith(
      "/auth/accounts/123/deactivate",
      "test-token",
      null,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<{ message: string }>>(response);
    expect(json.data.message).toBe("Account deactivated");
  });
});
