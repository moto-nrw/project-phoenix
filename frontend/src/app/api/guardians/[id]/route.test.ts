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

vi.mock("@/lib/api-helpers.server", async () => {
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

describe("GET /api/guardians/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/guardians/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("fetches guardian by ID from backend", async () => {
    const mockGuardian = {
      id: 123,
      person_id: 100,
      first_name: "John",
      last_name: "Doe",
    };
    mockApiGet.mockResolvedValueOnce({ data: mockGuardian });

    const request = createMockRequest("/api/guardians/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(mockApiGet).toHaveBeenCalledWith("/api/guardians/123", "test-token");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockGuardian>>(response);
    expect(json.data).toEqual(mockGuardian);
  });
});

describe("PUT /api/guardians/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/guardians/123", {
      method: "PUT",
      body: { first_name: "Jane" },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("updates guardian via backend", async () => {
    const updateBody = { first_name: "Jane" };
    const mockUpdatedGuardian = {
      id: 123,
      person_id: 100,
      first_name: "Jane",
      last_name: "Doe",
    };
    mockApiPut.mockResolvedValueOnce({ data: mockUpdatedGuardian });

    const request = createMockRequest("/api/guardians/123", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/guardians/123",
      "test-token",
      updateBody,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedGuardian>>(
        response,
      );
    expect(json.data.first_name).toBe("Jane");
  });
});

describe("DELETE /api/guardians/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/guardians/123", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("deletes guardian via backend and returns null", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/guardians/123", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/guardians/123",
      "test-token",
    );
    expect(response.status).toBe(204);
  });
});
