import { describe, it, expect, vi, beforeEach } from "vitest";
import { DELETE } from "./route";
import { ApiResponseError } from "~/lib/api-response-error.server";
import {
  type ExtendedSession,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockApiDelete } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiDelete: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async () => {
  const { mockHandleApiError } =
    await import("~/test/helpers/handle-api-error");
  return {
    apiGet: vi.fn(),
    apiPost: vi.fn(),
    apiPut: vi.fn(),
    apiDelete: mockApiDelete,
    handleApiError: vi.fn(mockHandleApiError),
  };
});

// ============================================================================
// Test Helpers
// ============================================================================

function createMockContext(
  params: Record<
    string,
    string | number | Record<string, unknown> | undefined
  > = {},
) {
  return { params: Promise.resolve(params) } as {
    params: Promise<Record<string, string | string[] | undefined>>;
  };
}

// ============================================================================
// Tests
// ============================================================================

describe("DELETE /api/invitations/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/invitations/123");
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("deletes invitation via backend", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/invitations/123");
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/auth/invitations/123",
      "test-token",
    );
    expect(response.status).toBe(204);
  });

  it("handles numeric ID from params", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/invitations/456");
    const response = await DELETE(request, createMockContext({ id: 456 }));

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/auth/invitations/456",
      "test-token",
    );
    expect(response.status).toBe(204);
  });

  it("handles invitationId fallback param", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/invitations/789");
    const response = await DELETE(
      request,
      createMockContext({ invitationId: "789" }),
    );

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/auth/invitations/789",
      "test-token",
    );
    expect(response.status).toBe(204);
  });

  it("returns 500 when ID is missing", async () => {
    const request = createMockRequest("/api/invitations/");
    const response = await DELETE(request, createMockContext({}));

    // TypeError is caught by wrapper and returns 500
    expect(response.status).toBe(500);
    expect(mockApiDelete).not.toHaveBeenCalled();
  });

  it("returns 500 when ID is invalid type", async () => {
    const request = createMockRequest("/api/invitations/invalid");
    const response = await DELETE(
      request,
      createMockContext({ id: { invalid: true } }),
    );

    // TypeError is caught by wrapper and returns 500
    expect(response.status).toBe(500);
    expect(mockApiDelete).not.toHaveBeenCalled();
  });

  it("forwards backend errors", async () => {
    mockApiDelete.mockRejectedValueOnce(new ApiResponseError(404, "Not found"));

    const request = createMockRequest("/api/invitations/999");
    const response = await DELETE(request, createMockContext({ id: "999" }));

    expect(response.status).toBe(404);
  });
});
