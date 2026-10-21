import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

const { mockAuth, mockFetchDashboardAnalytics } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockFetchDashboardAnalytics: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/dashboard-api.server", () => ({
  fetchDashboardAnalytics: mockFetchDashboardAnalytics,
}));

describe("GET /api/dashboard/analytics", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns dashboard analytics data", async () => {
    const analyticsData = {
      totalStudents: 150,
      activeStudents: 85,
      totalRooms: 12,
      occupiedRooms: 8,
      todayCheckIns: 120,
      todayCheckOuts: 35,
    };

    mockFetchDashboardAnalytics.mockResolvedValueOnce(analyticsData);

    const request = createMockRequest("/api/dashboard/analytics");
    const response = await GET(request, createMockContext());

    expect(mockFetchDashboardAnalytics).toHaveBeenCalledWith("test-token");
    expect(response.status).toBe(200);

    const json = (await response.json()) as { data: unknown };
    expect(json.data).toEqual(analyticsData);
  });
});
