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

describe("POST /api/active/visits/[id]/end", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 500 when id parameter is missing", async () => {
    const request = createMockRequest("/api/active/visits//end", {
      method: "POST",
    });

    const response = await POST(request, createMockContext({}));

    expect(response.status).toBe(500);
    expect(mockApiPost).not.toHaveBeenCalled();
  });

  it("returns 500 when id parameter is not a string", async () => {
    const request = createMockRequest("/api/active/visits/123/end", {
      method: "POST",
    });

    const response = await POST(
      request,
      createMockContext({ id: ["123", "456"] }),
    );

    expect(response.status).toBe(500);
    expect(mockApiPost).not.toHaveBeenCalled();
  });

  it("successfully ends a visit", async () => {
    const mockEndedVisit = {
      id: 123,
      student_id: 456,
      active_group_id: 1,
      start_time: "2024-01-15T09:00:00Z",
      end_time: "2024-01-15T17:00:00Z",
    };
    mockApiPost.mockResolvedValueOnce(mockEndedVisit);

    const request = createMockRequest("/api/active/visits/123/end", {
      method: "POST",
    });
    const response = await POST(request, createMockContext({ id: "123" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/active/visits/123/end",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockEndedVisit>>(response);
    expect(json.data).toEqual(mockEndedVisit);
    expect(json.data.end_time).toBe("2024-01-15T17:00:00Z");
  });

  it("handles visit already ended", async () => {
    mockApiPost.mockRejectedValueOnce(new ApiResponseError(400, ""));

    const request = createMockRequest("/api/active/visits/123/end", {
      method: "POST",
    });
    const response = await POST(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(400);
  });
});
