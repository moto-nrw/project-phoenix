import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET, POST } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiGet, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiGet: mockApiGet,
  apiPost: mockApiPost,
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
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

describe("GET /api/users", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("fetches users from backend", async () => {
    const mockUsers = {
      status: "success",
      data: [
        {
          id: 1,
          first_name: "John",
          last_name: "Doe",
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        },
        {
          id: 2,
          first_name: "Jane",
          last_name: "Smith",
          tag_id: "TAG123",
          created_at: "2024-01-02T00:00:00Z",
          updated_at: "2024-01-02T00:00:00Z",
        },
      ],
    };
    mockApiGet.mockResolvedValueOnce(mockUsers);

    const request = createMockRequest("/api/users");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith("/api/users", "test-token");
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toHaveLength(2);
  });

  it("handles query parameters", async () => {
    const mockUsers = { status: "success", data: [] };
    mockApiGet.mockResolvedValueOnce(mockUsers);

    const request = createMockRequest("/api/users?search=John");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/users?search=John",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("returns empty array when API returns null", async () => {
    mockApiGet.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/users");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });

  it("returns empty array when API returns unexpected structure", async () => {
    mockApiGet.mockResolvedValueOnce({ unexpected: "structure" });

    const request = createMockRequest("/api/users");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });

  it("propagates a failed backend request instead of returning an empty array", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Server error"));

    const request = createMockRequest("/api/users");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Server error");
  });
});

describe("POST /api/users", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("creates a new user", async () => {
    const createBody = { first_name: "John", last_name: "Doe" };
    const mockCreatedUser = {
      id: 1,
      first_name: "John",
      last_name: "Doe",
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-01T00:00:00Z",
    };
    mockApiPost.mockResolvedValueOnce(mockCreatedUser);

    const request = createMockRequest("/api/users", {
      method: "POST",
      body: createBody,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/users",
      "test-token",
      createBody,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<{ id: number; first_name: string }>>(
        response,
      );
    expect(json.data.first_name).toBe("John");
  });

  it("throws error when first_name is missing", async () => {
    const request = createMockRequest("/api/users", {
      method: "POST",
      body: { last_name: "Doe" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("first_name cannot be blank");
  });

  it("throws error when first_name is blank", async () => {
    const request = createMockRequest("/api/users", {
      method: "POST",
      body: { first_name: "  ", last_name: "Doe" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("first_name cannot be blank");
  });

  it("throws error when last_name is missing", async () => {
    const request = createMockRequest("/api/users", {
      method: "POST",
      body: { first_name: "John" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("last_name cannot be blank");
  });

  it("throws error when last_name is blank", async () => {
    const request = createMockRequest("/api/users", {
      method: "POST",
      body: { first_name: "John", last_name: "" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("last_name cannot be blank");
  });
});
