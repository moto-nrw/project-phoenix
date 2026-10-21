import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET, POST } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockApiGet, mockApiPost, mockMapActivityScheduleResponse } =
  vi.hoisted(() => ({
    mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
    mockApiGet: vi.fn(),
    mockApiPost: vi.fn(),
    mockMapActivityScheduleResponse: vi.fn(
      (data: {
        id: number;
        activity_id: number;
        weekday: number;
        timeframe_id: number;
      }) => ({
        id: data.id.toString(),
        activity_id: data.activity_id.toString(),
        weekday: data.weekday,
        timeframe_id: data.timeframe_id,
      }),
    ),
  }));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async (importOriginal) => {
  const { mockHandleApiError } =
    await import("~/test/helpers/handle-api-error");
  return {
    ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
    apiGet: mockApiGet,
    apiPost: mockApiPost,
    apiPut: vi.fn(),
    apiDelete: vi.fn(),
    handleApiError: vi.fn(mockHandleApiError),
  };
});

vi.mock("~/lib/activity-helpers", () => ({
  mapActivityScheduleResponse: mockMapActivityScheduleResponse,
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

describe("GET /api/activities/[id]/schedules", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/1/schedules");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("returns schedules with wrapped response", async () => {
    const schedules = [
      { id: 1, activity_id: 1, weekday: 1, timeframe_id: 1 },
      { id: 2, activity_id: 1, weekday: 3, timeframe_id: 2 },
    ];
    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: schedules,
    });

    const request = createMockRequest("/api/activities/1/schedules");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/activities/1/schedules",
      "test-token",
    );
    expect(mockMapActivityScheduleResponse).toHaveBeenCalledTimes(2);
    expect(response.status).toBe(200);
  });

  it("returns schedules with direct array response", async () => {
    const schedules = [{ id: 1, activity_id: 1, weekday: 1, timeframe_id: 1 }];
    mockApiGet.mockResolvedValueOnce(schedules);

    const request = createMockRequest("/api/activities/1/schedules");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(mockMapActivityScheduleResponse).toHaveBeenCalledTimes(1);
    expect(response.status).toBe(200);
  });

  it("throws error on unexpected response structure", async () => {
    mockApiGet.mockResolvedValueOnce({ unexpected: "structure" });

    const request = createMockRequest("/api/activities/1/schedules");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Unexpected response structure");
  });

  it("handles API errors", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("API error"));

    const request = createMockRequest("/api/activities/1/schedules");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Failed to fetch schedules");
  });
});

describe("POST /api/activities/[id]/schedules", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/1/schedules", {
      method: "POST",
      body: { weekday: "1", timeframe_id: 1 },
    });
    const response = await POST(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("creates schedule with wrapped response", async () => {
    const schedule = { id: 1, activity_id: 1, weekday: 1, timeframe_id: 1 };
    mockApiPost.mockResolvedValueOnce({
      status: "success",
      data: schedule,
    });

    const request = createMockRequest("/api/activities/1/schedules", {
      method: "POST",
      body: { weekday: "1", timeframe_id: 1 },
    });
    const response = await POST(request, createMockContext({ id: "1" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/activities/1/schedules",
      "test-token",
      { weekday: "1", timeframe_id: 1 },
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof schedule>>(response);
    expect(json.data).toEqual(schedule);
  });

  it("creates schedule with direct response", async () => {
    const schedule = { id: 1, activity_id: 1, weekday: 1, timeframe_id: 1 };
    mockApiPost.mockResolvedValueOnce(schedule);

    const request = createMockRequest("/api/activities/1/schedules", {
      method: "POST",
      body: { weekday: "1" },
    });
    const response = await POST(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof schedule>>(response);
    expect(json.data).toEqual(schedule);
  });

  it("throws error on unexpected response structure", async () => {
    mockApiPost.mockResolvedValueOnce({ unexpected: "structure" });

    const request = createMockRequest("/api/activities/1/schedules", {
      method: "POST",
      body: { weekday: "1" },
    });
    const response = await POST(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Unexpected response structure");
  });

  it("handles creation errors", async () => {
    mockApiPost.mockRejectedValueOnce(new Error("Creation failed"));

    const request = createMockRequest("/api/activities/1/schedules", {
      method: "POST",
      body: { weekday: "1" },
    });
    const response = await POST(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Creation failed");
  });
});
