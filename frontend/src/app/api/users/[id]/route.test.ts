import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiResponseError } from "~/lib/api-response-error.server";
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

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/users/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
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

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/users/1", {
      method: "PUT",
      body: { first_name: "Jane" },
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
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

  it("throws error when permission denied", async () => {
    mockApiPut.mockRejectedValueOnce(
      new ApiResponseError(
        403,
        JSON.stringify({
          type: "about:blank",
          title: "Forbidden",
          status: 403,
          detail: "users:update permission required",
          code: "general.permission",
        }),
      ),
    );

    const request = createMockRequest("/api/users/1", {
      method: "PUT",
      body: { first_name: "Jane" },
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(403);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toContain("users:update permission required");
  });

  it("throws error when user not found", async () => {
    mockApiPut.mockRejectedValueOnce(
      new ApiResponseError(
        404,
        JSON.stringify({
          type: "about:blank",
          title: "Not Found",
          status: 404,
          detail: "person not found",
          code: "general.input",
        }),
      ),
    );

    const request = createMockRequest("/api/users/1", {
      method: "PUT",
      body: { first_name: "Jane" },
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(404);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toContain("person not found");
  });
});

describe("DELETE /api/users/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/users/1", { method: "DELETE" });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("deletes user via backend and returns 204", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/users/1", { method: "DELETE" });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(mockApiDelete).toHaveBeenCalledWith("/api/users/1", "test-token");
    expect(response.status).toBe(204);
  });

  it("throws error when permission denied", async () => {
    mockApiDelete.mockRejectedValueOnce(
      new ApiResponseError(
        403,
        JSON.stringify({
          type: "about:blank",
          title: "Forbidden",
          status: 403,
          detail: "users:delete permission required",
          code: "general.permission",
        }),
      ),
    );

    const request = createMockRequest("/api/users/1", { method: "DELETE" });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(403);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toContain("users:delete permission required");
  });

  it("throws error when user not found", async () => {
    mockApiDelete.mockRejectedValueOnce(
      new ApiResponseError(
        404,
        JSON.stringify({
          type: "about:blank",
          title: "Not Found",
          status: 404,
          detail: "person not found",
          code: "general.input",
        }),
      ),
    );

    const request = createMockRequest("/api/users/1", { method: "DELETE" });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(404);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toContain("person not found");
  });
});
