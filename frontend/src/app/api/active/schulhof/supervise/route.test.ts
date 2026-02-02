import { describe, it, expect, vi, beforeEach } from "vitest";
import type { NextRequest } from "next/server";
import { POST } from "./route";

// Mock the api-helpers module
vi.mock("~/lib/api-helpers", () => ({
  apiPost: vi.fn(),
}));

// Route context type matching Next.js 15+
type MockRouteContext = {
  params: Promise<Record<string, string | string[] | undefined>>;
};

// Handler type for the POST route
type PostHandler = (
  request: NextRequest,
  body: unknown,
  token: string,
  params: Record<string, unknown>,
) => Promise<unknown>;

// Response type for JSON parsing
interface MockResponse {
  data: {
    action: string;
    supervision_id?: number;
    active_group_id: number;
  };
}

// Mock the route-wrapper module
vi.mock("~/lib/route-wrapper", () => ({
  createPostHandler: vi.fn((handler: PostHandler) => {
    return async (request: NextRequest, _context: MockRouteContext) => {
      // Simulate the wrapper behavior - extract token and body, call handler
      const token = "test-token";
      const body: unknown = await request.json();
      const params: Record<string, unknown> = {};
      const result: unknown = await handler(request, body, token, params);
      return new Response(JSON.stringify({ data: result }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    };
  }),
}));

import { apiPost } from "~/lib/api-helpers";

const mockedApiPost = vi.mocked(apiPost);

// Helper to create mock context
const createMockContext = (): MockRouteContext => ({
  params: Promise.resolve({}),
});

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

    const response = await POST(mockRequest, createMockContext());

    expect(mockedApiPost).toHaveBeenCalledWith(
      "/api/active/schulhof/supervise",
      "test-token",
      { action: "start" },
    );

    const responseData = (await response.json()) as MockResponse;
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

    const response = await POST(mockRequest, createMockContext());

    expect(mockedApiPost).toHaveBeenCalledWith(
      "/api/active/schulhof/supervise",
      "test-token",
      { action: "stop" },
    );

    const responseData = (await response.json()) as MockResponse;
    expect(responseData.data.action).toBe("stopped");
  });

  it("throws error for invalid action", async () => {
    const mockRequest = {
      json: () => Promise.resolve({ action: "invalid" }),
    } as unknown as NextRequest;

    await expect(POST(mockRequest, createMockContext())).rejects.toThrow(
      "Action must be 'start' or 'stop'",
    );

    expect(mockedApiPost).not.toHaveBeenCalled();
  });

  it("throws error when action is missing", async () => {
    const mockRequest = {
      json: () => Promise.resolve({}),
    } as unknown as NextRequest;

    await expect(POST(mockRequest, createMockContext())).rejects.toThrow(
      "Action must be 'start' or 'stop'",
    );

    expect(mockedApiPost).not.toHaveBeenCalled();
  });

  it("throws error when action is undefined", async () => {
    const mockRequest = {
      json: () => Promise.resolve({ action: undefined }),
    } as unknown as NextRequest;

    await expect(POST(mockRequest, createMockContext())).rejects.toThrow(
      "Action must be 'start' or 'stop'",
    );

    expect(mockedApiPost).not.toHaveBeenCalled();
  });

  it("handles apiPost errors gracefully", async () => {
    mockedApiPost.mockRejectedValueOnce(new Error("Backend error"));

    const mockRequest = {
      json: () => Promise.resolve({ action: "start" }),
    } as unknown as NextRequest;

    await expect(POST(mockRequest, createMockContext())).rejects.toThrow(
      "Backend error",
    );

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

    await expect(POST(mockRequestUpper, createMockContext())).rejects.toThrow(
      "Action must be 'start' or 'stop'",
    );

    const mockRequestMixed = {
      json: () => Promise.resolve({ action: "Start" }),
    } as unknown as NextRequest;

    await expect(POST(mockRequestMixed, createMockContext())).rejects.toThrow(
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

    await POST(mockRequest, createMockContext());

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

    const response = await POST(mockRequest, createMockContext());

    const responseData = (await response.json()) as MockResponse;
    expect(responseData.data.supervision_id).toBeUndefined();
    expect(responseData.data.action).toBe("stopped");
  });
});
