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

describe("GET /api/active/supervisors/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("fetches supervisor by ID", async () => {
    const mockSupervisor = { id: 1, staff_id: 10, active_group_id: 5 };
    mockApiGet.mockResolvedValueOnce(mockSupervisor);

    const request = createMockRequest("/api/active/supervisors/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/supervisors/1",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockSupervisor>>(response);
    expect(json.data).toEqual(mockSupervisor);
  });

  it("returns error when id is invalid", async () => {
    const request = createMockRequest("/api/active/supervisors/invalid");
    const response = await GET(request, createMockContext({ id: undefined }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Invalid id parameter");
  });
});

describe("PUT /api/active/supervisors/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("updates supervisor via backend", async () => {
    const updateBody = { active_group_id: "6" };
    const mockUpdatedSupervisor = {
      id: 1,
      staff_id: 10,
      active_group_id: 6,
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedSupervisor);

    const request = createMockRequest("/api/active/supervisors/1", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/active/supervisors/1",
      "test-token",
      updateBody,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedSupervisor>>(
        response,
      );
    expect(json.data).toEqual(mockUpdatedSupervisor);
  });

  it("returns error when id is invalid", async () => {
    const request = createMockRequest("/api/active/supervisors/invalid", {
      method: "PUT",
      body: { active_group_id: "6" },
    });
    const response = await PUT(request, createMockContext({ id: undefined }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Invalid id parameter");
  });
});

describe("DELETE /api/active/supervisors/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("deletes supervisor via backend and returns 204", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/active/supervisors/1", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/active/supervisors/1",
      "test-token",
    );
    expect(response.status).toBe(204);
  });

  it("returns error when id is invalid", async () => {
    const request = createMockRequest("/api/active/supervisors/invalid", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: undefined }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Invalid id parameter");
  });
});
