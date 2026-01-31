import { describe, it, expect, vi, beforeEach } from "vitest";
import type { NextRequest } from "next/server";
import { POST } from "./route";

// Mock the api-helpers module
vi.mock("~/lib/api-helpers", () => ({
  apiPost: vi.fn(),
}));

// Mock the route-wrapper module
vi.mock("~/lib/route-wrapper", () => ({
  createPostHandler: vi.fn((handler) => {
    return async (request: NextRequest) => {
      // Simulate the wrapper behavior - extract token and body, call handler
      const token = "test-token";
      const body = await request.json();
      const result = await handler(request, body, token);
      return new Response(JSON.stringify({ data: result }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    };
  }),
}));

import { apiPost } from "~/lib/api-helpers";

const mockedApiPost = vi.mocked(apiPost);

describe("POST /api/active/schulhof/supervise", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("starts supervision with valid action", async () => {
    const sampleBackendResponse = {
      action: "started",
      supervision_id: 456,
      active_group_id: 789,
    };

    mockedApiPost.mockResolvedValueOnce(sampleBackendResponse);

    const mockRequest = {
      json: () => Promise.resolve({ action: "start" }),
    } as unknown as NextRequest;

    const response = await POST(mockRequest);

    expect(mockedApiPost).toHaveBeenCalledWith(
      "/api/active/schulhof/supervise",
      "test-token",
      { action: "start" },
    );

    const responseData = await response.json();
    expect(responseData.data).toEqual(sampleBackendResponse);
  });

  it("stops supervision with valid action", async () => {
    const sampleBackendResponse = {
      action: "stopped",
      active_group_id: 789,
    };

    mockedApiPost.mockResolvedValueOnce(sampleBackendResponse);

    const mockRequest = {
      json: () => Promise.resolve({ action: "stop" }),
    } as unknown as NextRequest;

    const response = await POST(mockRequest);

    expect(mockedApiPost).toHaveBeenCalledWith(
      "/api/active/schulhof/supervise",
      "test-token",
      { action: "stop" },
    );

    const responseData = await response.json();
    expect(responseData.data.action).toBe("stopped");
  });

  it("throws error for invalid action", async () => {
    const mockRequest = {
      json: () => Promise.resolve({ action: "invalid" }),
    } as unknown as NextRequest;

    await expect(POST(mockRequest)).rejects.toThrow(
      "Action must be 'start' or 'stop'",
    );

    expect(mockedApiPost).not.toHaveBeenCalled();
  });

  it("throws error when action is missing", async () => {
    const mockRequest = {
      json: () => Promise.resolve({}),
    } as unknown as NextRequest;

    await expect(POST(mockRequest)).rejects.toThrow(
      "Action must be 'start' or 'stop'",
    );

    expect(mockedApiPost).not.toHaveBeenCalled();
  });

  it("throws error when action is undefined", async () => {
    const mockRequest = {
      json: () => Promise.resolve({ action: undefined }),
    } as unknown as NextRequest;

    await expect(POST(mockRequest)).rejects.toThrow(
      "Action must be 'start' or 'stop'",
    );

    expect(mockedApiPost).not.toHaveBeenCalled();
  });

  it("handles apiPost errors gracefully", async () => {
    mockedApiPost.mockRejectedValueOnce(new Error("Backend error"));

    const mockRequest = {
      json: () => Promise.resolve({ action: "start" }),
    } as unknown as NextRequest;

    await expect(POST(mockRequest)).rejects.toThrow("Backend error");

    expect(mockedApiPost).toHaveBeenCalledWith(
      "/api/active/schulhof/supervise",
      "test-token",
      { action: "start" },
    );
  });

  it("validates action is exactly 'start' or 'stop' (case sensitive)", async () => {
    const mockRequestUpper = {
      json: () => Promise.resolve({ action: "START" }),
    } as unknown as NextRequest;

    await expect(POST(mockRequestUpper)).rejects.toThrow(
      "Action must be 'start' or 'stop'",
    );

    const mockRequestMixed = {
      json: () => Promise.resolve({ action: "Start" }),
    } as unknown as NextRequest;

    await expect(POST(mockRequestMixed)).rejects.toThrow(
      "Action must be 'start' or 'stop'",
    );

    expect(mockedApiPost).not.toHaveBeenCalled();
  });

  it("passes action through to backend exactly as provided", async () => {
    const sampleBackendResponse = {
      action: "started",
      supervision_id: 123,
      active_group_id: 456,
    };

    mockedApiPost.mockResolvedValueOnce(sampleBackendResponse);

    const mockRequest = {
      json: () => Promise.resolve({ action: "start" }),
    } as unknown as NextRequest;

    await POST(mockRequest);

    // Verify the action is passed as-is in the request body
    expect(mockedApiPost).toHaveBeenCalledWith(
      "/api/active/schulhof/supervise",
      "test-token",
      { action: "start" },
    );
  });

  it("handles response when supervision_id is not present (stop action)", async () => {
    const sampleBackendResponse = {
      action: "stopped",
      active_group_id: 999,
      // supervision_id is undefined
    };

    mockedApiPost.mockResolvedValueOnce(sampleBackendResponse);

    const mockRequest = {
      json: () => Promise.resolve({ action: "stop" }),
    } as unknown as NextRequest;

    const response = await POST(mockRequest);

    const responseData = await response.json();
    expect(responseData.data.supervision_id).toBeUndefined();
    expect(responseData.data.action).toBe("stopped");
  });
});
