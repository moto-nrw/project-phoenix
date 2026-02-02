import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockFetchDashboardAnalytics } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockFetchDashboardAnalytics: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/dashboard-api", () => ({
  fetchDashboardAnalytics: mockFetchDashboardAnalytics,
}));

function createMockRequest(path: string): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  return new NextRequest(url);
}

function createMockContext(
  params: Record<string, string | string[] | undefined> = {},
) {
  return { params: Promise.resolve(params) };
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

describe("GET /api/dashboard/analytics", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/dashboard/analytics");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
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

  it("handles API errors gracefully", async () => {
    mockFetchDashboardAnalytics.mockRejectedValueOnce(
      new Error("Backend unavailable"),
    );

    const request = createMockRequest("/api/dashboard/analytics");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
  });
});
