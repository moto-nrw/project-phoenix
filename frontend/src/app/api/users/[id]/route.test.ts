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

vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: mockApiPut,
  apiDelete: mockApiDelete,
  handleApiError: vi.fn(
    (await import("~/test/helpers/handle-api-error")).mockHandleApiError,
  ),
}));

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

describe("GET /api/users/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("fetches user by ID from backend", async () => {
    const mockUser = {
      status: "success",
      data: {
        id: 1,
        first_name: "John",
        last_name: "Doe",
        tag_id: "TAG123",
        account_id: 10,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
    };
    mockApiGet.mockResolvedValueOnce(mockUser);

    const request = createMockRequest("/api/users/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(mockApiGet).toHaveBeenCalledWith("/api/users/1", "test-token");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<{ id: number; first_name: string }>>(
        response,
      );
    expect(json.data.id).toBe(1);
    expect(json.data.first_name).toBe("John");
  });

  it("throws error when user not found", async () => {
    mockApiGet.mockResolvedValueOnce({ data: null });

    const request = createMockRequest("/api/users/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Person not found");
  });

  it("throws error when ID is missing", async () => {
    const request = createMockRequest("/api/users/undefined");
    const response = await GET(request, createMockContext({}));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Person ID is required");
  });
});

describe("PUT /api/users/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("updates user via backend", async () => {
    const updateBody = { first_name: "Jane", last_name: "Doe" };
    const mockUpdatedUser = {
      id: 1,
      first_name: "Jane",
      last_name: "Doe",
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-15T10:00:00Z",
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedUser);

    const request = createMockRequest("/api/users/1", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/users/1",
      "test-token",
      updateBody,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<{ first_name: string }>>(response);
    expect(json.data.first_name).toBe("Jane");
  });

  it("throws error when first_name is blank", async () => {
    const request = createMockRequest("/api/users/1", {
      method: "PUT",
      body: { first_name: "  " },
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("First name cannot be blank");
  });

  it("throws error when last_name is blank", async () => {
    const request = createMockRequest("/api/users/1", {
      method: "PUT",
      body: { last_name: "" },
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Last name cannot be blank");
  });
});

describe("DELETE /api/users/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("deletes user via backend and returns 204", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/users/1", { method: "DELETE" });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(mockApiDelete).toHaveBeenCalledWith("/api/users/1", "test-token");
    expect(response.status).toBe(204);
  });
});
