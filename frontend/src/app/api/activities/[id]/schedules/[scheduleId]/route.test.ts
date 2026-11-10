import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks
// ============================================================================

const {
  mockAuth,
  mockApiGet,
  mockApiPut,
  mockApiDelete,
  mockMapActivityScheduleResponse,
} = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPut: vi.fn(),
  mockApiDelete: vi.fn(),
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

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
  }),
);

vi.mock("~/lib/api-helpers.server", async (importOriginal) => {
  const { mockHandleApiError } =
    await import("~/test/helpers/handle-api-error");
  return {
    ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
    apiGet: mockApiGet,
    apiPost: vi.fn(),
    apiPut: mockApiPut,
    apiDelete: mockApiDelete,
    handleApiError: vi.fn(mockHandleApiError),
  };
});

vi.mock("~/lib/activity-helpers", () => ({
  mapActivityScheduleResponse: mockMapActivityScheduleResponse,
}));

// ============================================================================
// Test Helpers
// ============================================================================

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/activities/[id]/schedules/[scheduleId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns schedule with wrapped response", async () => {
    const schedule = { id: 10, activity_id: 1, weekday: 1, timeframe_id: 1 };
    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: schedule,
    });

    const request = createMockRequest("/api/activities/1/schedules/10");
    const response = await GET(
      request,
      createMockContext({ id: "1", scheduleId: "10" }),
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/activities/1/schedules/10",
      "test-token",
    );
    expect(mockMapActivityScheduleResponse).toHaveBeenCalledWith(schedule);
    expect(response.status).toBe(200);
  });

  it("returns schedule with direct response", async () => {
    const schedule = { id: 10, activity_id: 1, weekday: 1, timeframe_id: 1 };
    mockApiGet.mockResolvedValueOnce(schedule);

    const request = createMockRequest("/api/activities/1/schedules/10");
    const response = await GET(
      request,
      createMockContext({ id: "1", scheduleId: "10" }),
    );

    expect(mockMapActivityScheduleResponse).toHaveBeenCalledWith(schedule);
    expect(response.status).toBe(200);
  });

  it("throws error on unexpected response structure", async () => {
    mockApiGet.mockResolvedValueOnce({ unexpected: "structure" });

    const request = createMockRequest("/api/activities/1/schedules/10");
    const response = await GET(
      request,
      createMockContext({ id: "1", scheduleId: "10" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Unexpected response structure");
  });

  it("handles API errors", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Schedule not found"));

    const request = createMockRequest("/api/activities/1/schedules/10");
    const response = await GET(
      request,
      createMockContext({ id: "1", scheduleId: "10" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Failed to fetch schedule");
  });
});
