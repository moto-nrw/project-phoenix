import { describe, it, expect, vi, beforeEach } from "vitest";
import type { NextRequest } from "next/server";
import { GET } from "./route";

// Mock the api-helpers module
vi.mock("~/lib/api-helpers", () => ({
  apiGet: vi.fn(),
}));

// Mock the route-wrapper module
vi.mock("~/lib/route-wrapper", () => ({
  createGetHandler: vi.fn((handler) => {
    return async (request: NextRequest) => {
      // Simulate the wrapper behavior - extract token and call handler
      const token = "test-token";
      const result = await handler(request, token);
      return new Response(JSON.stringify({ data: result }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    };
  }),
}));

import { apiGet } from "~/lib/api-helpers";

const mockedApiGet = vi.mocked(apiGet);

describe("GET /api/active/schulhof/status", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls apiGet with correct endpoint and token", async () => {
    const sampleBackendStatus = {
      exists: true,
      room_id: 42,
      room_name: "Schulhof",
      activity_group_id: 10,
      active_group_id: 5,
      is_user_supervising: true,
      supervision_id: 123,
      supervisor_count: 2,
      student_count: 15,
      supervisors: [
        {
          id: 1,
          staff_id: 100,
          name: "Frau Schmidt",
          is_current_user: true,
        },
      ],
    };

    mockedApiGet.mockResolvedValueOnce(sampleBackendStatus);

    const mockRequest = {} as NextRequest;
    const response = await GET(mockRequest);

    expect(mockedApiGet).toHaveBeenCalledWith(
      "/api/active/schulhof/status",
      "test-token",
    );

    const responseData = await response.json();
    expect(responseData.data).toEqual(sampleBackendStatus);
  });

  it("returns non-existent Schulhof status", async () => {
    const sampleBackendStatus = {
      exists: false,
      room_name: "",
      is_user_supervising: false,
      supervisor_count: 0,
      student_count: 0,
      supervisors: [],
    };

    mockedApiGet.mockResolvedValueOnce(sampleBackendStatus);

    const mockRequest = {} as NextRequest;
    const response = await GET(mockRequest);

    expect(mockedApiGet).toHaveBeenCalledWith(
      "/api/active/schulhof/status",
      "test-token",
    );

    const responseData = await response.json();
    expect(responseData.data.exists).toBe(false);
  });

  it("handles apiGet errors gracefully", async () => {
    mockedApiGet.mockRejectedValueOnce(new Error("Backend error"));

    const mockRequest = {} as NextRequest;

    await expect(GET(mockRequest)).rejects.toThrow("Backend error");

    expect(mockedApiGet).toHaveBeenCalledWith(
      "/api/active/schulhof/status",
      "test-token",
    );
  });

  it("returns complete Schulhof status with multiple supervisors", async () => {
    const sampleBackendStatus = {
      exists: true,
      room_id: 99,
      room_name: "Schulhof",
      activity_group_id: 20,
      active_group_id: 30,
      is_user_supervising: false,
      supervision_id: undefined,
      supervisor_count: 3,
      student_count: 42,
      supervisors: [
        {
          id: 1,
          staff_id: 100,
          name: "Frau Schmidt",
          is_current_user: false,
        },
        {
          id: 2,
          staff_id: 101,
          name: "Herr Müller",
          is_current_user: false,
        },
        {
          id: 3,
          staff_id: 102,
          name: "Frau Weber",
          is_current_user: false,
        },
      ],
    };

    mockedApiGet.mockResolvedValueOnce(sampleBackendStatus);

    const mockRequest = {} as NextRequest;
    const response = await GET(mockRequest);

    const responseData = await response.json();
    expect(responseData.data.supervisors).toHaveLength(3);
    expect(responseData.data.student_count).toBe(42);
  });
});
