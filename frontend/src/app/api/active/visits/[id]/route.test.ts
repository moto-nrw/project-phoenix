import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET, PUT, DELETE } from "./route";
import { mockServerSessionData } from "~/test/mocks/next-auth";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiGet, mockApiPut, mockApiDelete } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPut: vi.fn(),
  mockApiDelete: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async () => {
  const { mockHandleApiError } =
    await import("~/test/helpers/handle-api-error");
  return {
    apiGet: mockApiGet,
    apiPost: vi.fn(),
    apiPut: mockApiPut,
    apiDelete: mockApiDelete,
    handleApiError: vi.fn(mockHandleApiError),
  };
});

// ============================================================================
// Test Helpers
// ============================================================================

const defaultSession = mockServerSessionData() as ExtendedSession;

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

describe("GET /api/active/visits/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("fetches visit by ID from backend", async () => {
    const mockVisit = {
      id: 123,
      student_id: 1,
      active_group_id: 1,
      start_time: "2024-01-15T09:00:00Z",
    };
    mockApiGet.mockResolvedValueOnce(mockVisit);

    const request = createMockRequest("/api/active/visits/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/visits/123",
      "test-token",
    );
    expect(response.status).toBe(200);
  });
});

describe("PUT /api/active/visits/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("updates visit via backend", async () => {
    const updateBody = { end_time: "2024-01-15T17:00:00Z" };
    const mockUpdatedVisit = {
      id: 123,
      student_id: 1,
      active_group_id: 1,
      start_time: "2024-01-15T09:00:00Z",
      end_time: "2024-01-15T17:00:00Z",
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedVisit);

    const request = createMockRequest("/api/active/visits/123", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/active/visits/123",
      "test-token",
      updateBody,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedVisit>>(response);
    expect(json.data.end_time).toBe("2024-01-15T17:00:00Z");
  });
});

describe("DELETE /api/active/visits/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("deletes visit via backend and returns 204", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/active/visits/123", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/active/visits/123",
      "test-token",
    );
    expect(response.status).toBe(204);
  });
});
