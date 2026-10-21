import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET, POST } from "./route";
import { ApiResponseError } from "~/lib/api-response-error.server";
import { suppressConsole } from "~/test/helpers/console";
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

describe("GET /api/rooms", () => {
  const consoleSpies = suppressConsole("warn");

  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("fetches rooms from backend API", async () => {
    const mockRooms = [
      { id: 1, name: "Room A", is_occupied: false },
      { id: 2, name: "Room B", is_occupied: true },
    ];
    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: mockRooms,
      pagination: {
        current_page: 1,
        page_size: 50,
        total_pages: 1,
        total_records: 2,
      },
    });

    const request = createMockRequest("/api/rooms");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith("/api/rooms", "test-token");
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      data: typeof mockRooms;
      pagination: {
        current_page: number;
        page_size: number;
        total_pages: number;
        total_records: number;
      };
      status: string;
    }>(response);
    expect(json.data).toEqual(mockRooms);
  });

  it("forwards query parameters to backend", async () => {
    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: [],
      pagination: {
        current_page: 1,
        page_size: 50,
        total_pages: 0,
        total_records: 0,
      },
    });

    const request = createMockRequest("/api/rooms?category=classroom&floor=2");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/rooms?category=classroom&floor=2",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("returns empty data when API returns null", async () => {
    mockApiGet.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/rooms");
    const response = await GET(request, createMockContext());

    const json = await parseJsonResponse<unknown[]>(response);
    expect(json).toEqual([]);
  });

  it("does not turn a failed backend request into an empty room list", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/rooms");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Network error");
  });

  it("logs rate-limited fetch failures as warnings", async () => {
    mockApiGet.mockRejectedValueOnce(
      new ApiResponseError(429, "Rate limit exceeded"),
    );

    const request = createMockRequest("/api/rooms");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(429);
    expect(consoleSpies.warn).toHaveBeenCalledWith("api route rate limited", {
      error: "Rate limit exceeded",
      rate_limited: true,
      status: 429,
    });
  });
});

describe("POST /api/rooms", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("creates a new room via backend API", async () => {
    const createRequest = {
      name: "New Room",
      building: "Building A",
      floor: 2,
      capacity: 30,
    };
    const mockCreatedRoom = {
      id: 1,
      name: "New Room",
      building: "Building A",
      floor: 2,
      capacity: 30,
      is_occupied: false,
    };
    mockApiPost.mockResolvedValueOnce(mockCreatedRoom);

    const request = createMockRequest("/api/rooms", {
      method: "POST",
      body: createRequest,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/rooms",
      "test-token",
      createRequest,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockCreatedRoom>>(response);
    expect(json.data).toEqual(mockCreatedRoom);
  });

  it("rejects room creation with empty name", async () => {
    const request = createMockRequest("/api/rooms", {
      method: "POST",
      body: { name: "" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("name cannot be blank");
  });

  it("rejects room creation with invalid capacity", async () => {
    const request = createMockRequest("/api/rooms", {
      method: "POST",
      body: { name: "Room", capacity: 0 },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain(
      "Capacity must be greater than 0",
    );
  });
});
