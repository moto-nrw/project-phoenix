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

vi.mock("~/lib/api-helpers.server", async () => {
  const { mockHandleApiError } =
    await import("~/test/helpers/handle-api-error");
  return {
    apiGet: mockApiGet,
    apiPost: mockApiPost,
    apiPut: vi.fn(),
    apiDelete: vi.fn(),
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

describe("GET /api/groups", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("fetches groups from backend API", async () => {
    const mockGroups = [
      { id: 1, name: "Group A" },
      { id: 2, name: "Group B" },
    ];
    mockApiGet.mockResolvedValueOnce({ data: mockGroups });

    const request = createMockRequest("/api/groups");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith("/api/groups", "test-token");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockGroups>>(response);
    expect(json.data).toEqual(mockGroups);
  });

  it("forwards query parameters to backend", async () => {
    mockApiGet.mockResolvedValueOnce({ data: [] });

    const request = createMockRequest("/api/groups?type=ogs&status=active");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/groups?type=ogs&status=active",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("returns empty array when backend returns no data", async () => {
    mockApiGet.mockResolvedValueOnce({ data: null });

    const request = createMockRequest("/api/groups");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });
});

describe("POST /api/groups", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("creates a new group via backend API", async () => {
    const createRequest = {
      name: "New Group",
      description: "Test group",
      room_id: 10,
    };
    const mockCreatedGroup = {
      id: 1,
      name: "New Group",
      description: "Test group",
      room_id: 10,
    };
    mockApiPost.mockResolvedValueOnce(mockCreatedGroup);

    const request = createMockRequest("/api/groups", {
      method: "POST",
      body: createRequest,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/groups",
      "test-token",
      createRequest,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockCreatedGroup>>(response);
    expect(json.data).toEqual(mockCreatedGroup);
  });

  it("creates group with minimal required fields", async () => {
    const createRequest = { name: "Minimal Group" };
    const mockCreatedGroup = { id: 2, name: "Minimal Group" };
    mockApiPost.mockResolvedValueOnce(mockCreatedGroup);

    const request = createMockRequest("/api/groups", {
      method: "POST",
      body: createRequest,
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(200);
    const json =
      await parseJsonResponse<ApiResponse<typeof mockCreatedGroup>>(response);
    expect(json.data.name).toBe("Minimal Group");
  });
});
