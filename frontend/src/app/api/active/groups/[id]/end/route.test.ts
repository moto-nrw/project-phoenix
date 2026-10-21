import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { POST } from "./route";
import { ApiResponseError } from "~/lib/api-response-error.server";
import {
  type ExtendedSession,
  createMockContext,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async () => {
  const { mockHandleApiError } =
    await import("~/test/helpers/handle-api-error");
  return {
    apiGet: vi.fn(),
    apiPost: mockApiPost,
    apiPut: vi.fn(),
    apiDelete: vi.fn(),
    handleApiError: vi.fn(mockHandleApiError),
  };
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
      method: options.method ?? "POST",
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

describe("POST /api/active/groups/[id]/end", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/groups/123/end", {
      method: "POST",
    });
    const response = await POST(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("ends an active group successfully", async () => {
    const mockEndedGroup = {
      id: 123,
      name: "OGS Group A",
      ended_at: "2024-01-15T17:00:00Z",
    };
    mockApiPost.mockResolvedValueOnce(mockEndedGroup);

    const request = createMockRequest("/api/active/groups/123/end", {
      method: "POST",
    });
    const response = await POST(request, createMockContext({ id: "123" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/active/groups/123/end",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockEndedGroup>>(response);
    expect(json.data).toEqual(mockEndedGroup);
  });

  it("returns 404 when group not found", async () => {
    mockApiPost.mockRejectedValueOnce(
      new ApiResponseError(404, "Group not found"),
    );

    const request = createMockRequest("/api/active/groups/999/end", {
      method: "POST",
    });
    const response = await POST(request, createMockContext({ id: "999" }));

    expect(response.status).toBe(404);
  });

  it("returns 500 when id parameter is invalid", async () => {
    const request = createMockRequest("/api/active/groups/invalid/end", {
      method: "POST",
    });
    const response = await POST(request, createMockContext({ id: undefined }));

    expect(response.status).toBe(500);
  });
});
