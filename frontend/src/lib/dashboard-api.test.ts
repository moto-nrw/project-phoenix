import { describe, it, expect, vi, beforeEach } from "vitest";
import { fetchDashboardAnalyticsClient } from "./dashboard-api";
import { fetchDashboardAnalytics } from "./dashboard-api.server";
import { apiGet } from "./api-helpers.server";
import { mapDashboardAnalyticsResponse } from "./dashboard-helpers";
import { fetchWithAuth } from "./fetch-with-auth";

vi.mock("./api-helpers.server", () => ({
  apiGet: vi.fn(),
}));

vi.mock("./dashboard-helpers", () => ({
  mapDashboardAnalyticsResponse: vi.fn((data: unknown) => ({
    ...(data as Record<string, unknown>),
    mapped: true,
  })),
}));

vi.mock("./fetch-with-auth", () => ({
  fetchWithAuth: vi.fn(),
}));

function mockDashboardResponses(dashboardResponse: { data: unknown }) {
  vi.mocked(apiGet).mockImplementation(() =>
    Promise.resolve(dashboardResponse),
  );
}

describe("fetchDashboardAnalytics", () => {
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    vi.clearAllMocks();
    consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);
  });

  it("successfully fetches and maps dashboard analytics", async () => {
    const mockBackendData = {
      students_in_house: 42,
      students_in_wc: 5,
      students_in_school_yard: 10,
      students_home: 33,
      active_groups: 3,
      rooms_occupied: 8,
    };

    const mockResponse = {
      data: mockBackendData,
    };

    mockDashboardResponses(mockResponse);

    const token = "test-jwt-token";
    const result = await fetchDashboardAnalytics(token);

    expect(apiGet).toHaveBeenCalledWith(
      "/api/active/analytics/dashboard",
      token,
    );
    expect(mapDashboardAnalyticsResponse).toHaveBeenCalledWith(mockBackendData);
    expect(result).toEqual({
      ...mockBackendData,
      mapped: true,
    });
  });

  it("calls apiGet with correct endpoint and token", async () => {
    const mockResponse = {
      data: {
        students_in_house: 0,
        students_in_wc: 0,
        students_in_school_yard: 0,
        active_groups: 0,
        rooms_occupied: 0,
      },
    };

    mockDashboardResponses(mockResponse);

    const token = "my-secret-token";
    await fetchDashboardAnalytics(token);

    expect(apiGet).toHaveBeenCalledWith(
      "/api/active/analytics/dashboard",
      token,
    );
    // Exactly one call, and never /api/students: that endpoint is gated on
    // users:read while this one needs only groups:read, so pulling counts
    // from it would 403 the whole dashboard for a groups-only role.
    expect(apiGet).toHaveBeenCalledTimes(1);
    expect(apiGet).not.toHaveBeenCalledWith(
      expect.stringContaining("/api/students"),
      expect.anything(),
    );
  });

  it("extracts data from response wrapper", async () => {
    const innerData = {
      students_in_house: 100,
      students_in_wc: 2,
      students_in_school_yard: 15,
      active_groups: 5,
      rooms_occupied: 12,
    };

    const mockResponse = {
      data: innerData,
    };

    mockDashboardResponses(mockResponse);

    await fetchDashboardAnalytics("token");

    expect(mapDashboardAnalyticsResponse).toHaveBeenCalledWith(innerData);
  });

  it("re-throws error when API call fails", async () => {
    const error = new Error("Network error");
    vi.mocked(apiGet).mockRejectedValue(error);

    const token = "test-token";

    await expect(fetchDashboardAnalytics(token)).rejects.toThrow(
      "Network error",
    );

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "failed to fetch dashboard analytics",
      { error: String(error) },
    );
  });

  it("preserves 401 status error", async () => {
    const authError = {
      message: "Unauthorized",
      status: 401,
    };
    vi.mocked(apiGet).mockRejectedValue(authError);

    const token = "expired-token";

    await expect(fetchDashboardAnalytics(token)).rejects.toEqual(authError);

    expect(consoleErrorSpy).toHaveBeenCalled();
  });

  it("logs error before re-throwing", async () => {
    const error = new Error("API error");
    vi.mocked(apiGet).mockRejectedValue(error);

    try {
      await fetchDashboardAnalytics("token");
    } catch {
      // Expected to throw
    }

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "failed to fetch dashboard analytics",
      { error: String(error) },
    );
  });

  it("calls mapper function with correct data structure", async () => {
    const expectedData = {
      students_in_house: 25,
      students_in_wc: 3,
      students_in_school_yard: 8,
      active_groups: 2,
      rooms_occupied: 6,
    };

    mockDashboardResponses({ data: expectedData });

    await fetchDashboardAnalytics("token");

    expect(mapDashboardAnalyticsResponse).toHaveBeenCalledWith(expectedData);
    expect(mapDashboardAnalyticsResponse).toHaveBeenCalledTimes(1);
  });
});

describe("fetchDashboardAnalyticsClient", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("successfully fetches and returns dashboard analytics data", async () => {
    const mockData = {
      studentsPresent: 42,
      studentsInRooms: 30,
      studentsInTransit: 12,
    };

    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: mockData }),
    } as Response);

    const result = await fetchDashboardAnalyticsClient();

    expect(fetchWithAuth).toHaveBeenCalledWith("/api/dashboard/analytics");
    expect(result).toEqual(mockData);
  });

  it("throws error when response is not ok", async () => {
    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: false,
      status: 500,
    } as Response);

    await expect(fetchDashboardAnalyticsClient()).rejects.toThrow(
      "Dashboard fetch failed: 500",
    );
  });

  it("extracts data property from JSON response", async () => {
    const innerData = { studentsPresent: 100, freeRooms: 5 };

    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: innerData }),
    } as Response);

    const result = await fetchDashboardAnalyticsClient();

    expect(result).toEqual(innerData);
  });
});
