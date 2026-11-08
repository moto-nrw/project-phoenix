import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { POST } from "./route";
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

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
    withTenantAuth: (handler: unknown) => handler,
  }),
);

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

describe("POST /api/active/supervisors/[id]/end", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("ends supervision for a supervisor", async () => {
    const mockEndedSupervisor = {
      id: 1,
      staff_id: 10,
      active_group_id: 5,
      ended_at: "2024-01-15T17:00:00Z",
    };
    mockApiPost.mockResolvedValueOnce(mockEndedSupervisor);

    const request = createMockRequest("/api/active/supervisors/1/end");
    const response = await POST(request, createMockContext({ id: "1" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/active/supervisors/1/end",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockEndedSupervisor>>(
        response,
      );
    expect(json.data).toEqual(mockEndedSupervisor);
  });

  it("returns error when id is invalid", async () => {
    const request = createMockRequest("/api/active/supervisors/invalid/end");
    const response = await POST(request, createMockContext({ id: undefined }));

    // An invalid route param is the request's mistake: a coded 400.
    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Invalid id parameter");
  });

  it("returns error when id is not a string", async () => {
    const request = createMockRequest("/api/active/supervisors/1/end");
    const response = await POST(request, createMockContext({ id: ["1", "2"] }));

    // An invalid route param is the request's mistake: a coded 400.
    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Invalid id parameter");
  });
});
