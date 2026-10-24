import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET, PUT, DELETE } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
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

describe("GET /api/groups/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("fetches group by ID from backend", async () => {
    const mockGroup = {
      id: 123,
      name: "Test Group",
      description: "A test group",
    };
    mockApiGet.mockResolvedValueOnce({ status: "success", data: mockGroup });

    const request = createMockRequest("/api/groups/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(mockApiGet).toHaveBeenCalledWith("/api/groups/123", "test-token");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockGroup>>(response);
    expect(json.data).toEqual(mockGroup);
  });

  it("handles group not found error", async () => {
    mockApiGet.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/groups/999");
    const response = await GET(request, createMockContext({ id: "999" }));

    expect(response.status).toBe(500);
  });
});

describe("PUT /api/groups/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("updates group via backend", async () => {
    const updateBody = { name: "Updated Group", description: "New desc" };
    const mockUpdatedGroup = {
      id: 123,
      name: "Updated Group",
      description: "New desc",
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedGroup);

    const request = createMockRequest("/api/groups/123", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/groups/123",
      "test-token",
      updateBody,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedGroup>>(response);
    expect(json.data.name).toBe("Updated Group");
  });

  it("updates only specific fields", async () => {
    const updateBody = { room_id: 42 };
    const mockUpdatedGroup = { id: 123, name: "Group", room_id: 42 };
    mockApiPut.mockResolvedValueOnce(mockUpdatedGroup);

    const request = createMockRequest("/api/groups/123", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(200);
    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedGroup>>(response);
    expect(json.data.room_id).toBe(42);
  });
});

describe("DELETE /api/groups/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("deletes group via backend and returns 204", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/groups/123", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(mockApiDelete).toHaveBeenCalledWith("/api/groups/123", "test-token");
    expect(response.status).toBe(204);
  });
});
