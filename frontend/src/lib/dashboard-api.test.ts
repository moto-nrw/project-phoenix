import { describe, it, expect, vi, beforeEach } from "vitest";
import { fetchDashboardAnalytics } from "./dashboard-api";
import { apiGet } from "./api-helpers";
import { mapDashboardAnalyticsResponse } from "./dashboard-helpers";

vi.mock("./api-helpers", () => ({
  apiGet: vi.fn(),
}));

vi.mock("./dashboard-helpers", () => ({
   
  mapDashboardAnalyticsResponse: vi.fn((data: unknown) => ({
    ...(data as Record<string, unknown>),
    mapped: true,
  })),
}));

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
      active_groups: 3,
      rooms_occupied: 8,
    };

    const mockResponse = {
      data: mockBackendData,
    };

    vi.mocked(apiGet).mockResolvedValue(mockResponse);

    const token = "test-jwt-token";
    const result = await fetchDashboardAnalytics(token);

    expect(apiGet).toHaveBeenCalledWith(
      "/api/active/analytics/dashboard",
      token,
    );
    expect(mapDashboardAnalyticsResponse).toHaveBeenCalledWith(mockBackendData);
    expect(result).toEqual({ ...mockBackendData, mapped: true });
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

    vi.mocked(apiGet).mockResolvedValue(mockResponse);

    const token = "my-secret-token";
    await fetchDashboardAnalytics(token);

    expect(apiGet).toHaveBeenCalledWith(
      "/api/active/analytics/dashboard",
      token,
    );
    expect(apiGet).toHaveBeenCalledTimes(1);
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

    vi.mocked(apiGet).mockResolvedValue(mockResponse);

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
      "Error fetching dashboard analytics:",
      error,
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
      "Error fetching dashboard analytics:",
      error,
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

    vi.mocked(apiGet).mockResolvedValue({ data: expectedData });

    await fetchDashboardAnalytics("token");

    expect(mapDashboardAnalyticsResponse).toHaveBeenCalledWith(expectedData);
    expect(mapDashboardAnalyticsResponse).toHaveBeenCalledTimes(1);
  });
});
