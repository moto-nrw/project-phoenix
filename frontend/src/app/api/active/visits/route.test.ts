import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
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
    extractParams: vi.fn((request: NextRequest) => {
      const params = new URLSearchParams(request.nextUrl.search);
      const result: Record<string, string> = {};
      params.forEach((value, key) => {
        result[key] = value;
      });
      return result;
    }),
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

describe("GET /api/active/visits", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("fetches all active visits", async () => {
    const mockVisits = [
      {
        id: 1,
        student_id: 10,
        active_group_id: 1,
        start_time: "2024-01-15T09:00:00Z",
      },
      {
        id: 2,
        student_id: 11,
        active_group_id: 1,
        start_time: "2024-01-15T09:15:00Z",
      },
    ];
    mockApiGet.mockResolvedValueOnce({ data: mockVisits });

    const request = createMockRequest("/api/active/visits");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith("/api/active/visits", "test-token");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockVisits>>(response);
    expect(json.data).toEqual(mockVisits);
  });

  it("supports query parameters for filtering", async () => {
    const mockVisits = [
      {
        id: 1,
        student_id: 10,
        active_group_id: 5,
        start_time: "2024-01-15T09:00:00Z",
      },
    ];
    mockApiGet.mockResolvedValueOnce({ data: mockVisits });

    const request = createMockRequest("/api/active/visits?active_group_id=5");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/visits?active_group_id=5",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("supports multiple query parameters", async () => {
    const mockVisits = [
      {
        id: 1,
        student_id: 10,
        active_group_id: 5,
        start_time: "2024-01-15T09:00:00Z",
      },
    ];
    mockApiGet.mockResolvedValueOnce({ data: mockVisits });

    const request = createMockRequest(
      "/api/active/visits?active_group_id=5&student_id=10",
    );
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/visits?active_group_id=5&student_id=10",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("returns empty array when no visits exist", async () => {
    mockApiGet.mockResolvedValueOnce({ data: [] });

    const request = createMockRequest("/api/active/visits");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });

  it("handles backend response without data wrapper", async () => {
    const mockVisits = [
      {
        id: 1,
        student_id: 10,
        active_group_id: 1,
        start_time: "2024-01-15T09:00:00Z",
      },
    ];
    mockApiGet.mockResolvedValueOnce(mockVisits);

    const request = createMockRequest("/api/active/visits");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockVisits>>(response);
    expect(json.data).toEqual(mockVisits);
  });
});

describe("POST /api/active/visits", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("creates a new visit", async () => {
    const createRequest = {
      student_id: "10",
      active_group_id: "1",
      start_time: "2024-01-15T09:00:00Z",
    };
    const mockCreatedVisit = {
      id: 99,
      student_id: 10,
      active_group_id: 1,
      start_time: "2024-01-15T09:00:00Z",
    };
    mockApiPost.mockResolvedValueOnce(mockCreatedVisit);

    const request = createMockRequest("/api/active/visits", {
      method: "POST",
      body: createRequest,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/active/visits",
      "test-token",
      createRequest,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockCreatedVisit>>(response);
    expect(json.data).toEqual(mockCreatedVisit);
  });

  it("creates visit with minimal fields", async () => {
    const createRequest = {
      student_id: "10",
      active_group_id: "1",
    };
    const mockCreatedVisit = {
      id: 88,
      student_id: 10,
      active_group_id: 1,
      start_time: "2024-01-15T09:00:00Z",
    };
    mockApiPost.mockResolvedValueOnce(mockCreatedVisit);

    const request = createMockRequest("/api/active/visits", {
      method: "POST",
      body: createRequest,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/active/visits",
      "test-token",
      createRequest,
    );
    expect(response.status).toBe(200);
  });
});
