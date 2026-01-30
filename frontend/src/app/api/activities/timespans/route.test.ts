import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)")
      ? 401
      : message.includes("(404)")
        ? 404
        : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
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

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("GET /api/activities/timespans", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/timespans");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches timespans successfully with status: success wrapper", async () => {
    const mockTimespans = [
      {
        id: 1,
        name: "Morning Session",
        start_time: "08:00:00",
        end_time: "12:00:00",
        description: "Morning activities",
      },
      {
        id: 2,
        name: "Afternoon Session",
        start_time: "13:00:00",
        end_time: "17:00:00",
        description: "Afternoon activities",
      },
    ];

    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: mockTimespans,
    });

    const request = createMockRequest("/api/activities/timespans");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/activities/timespans",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{
        id: string;
        name: string;
        start_time: string;
        end_time: string;
      }>;
    }>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]?.id).toBe("1");
    expect(json.data[0]?.name).toBe("Morning Session");
    expect(json.data[0]?.start_time).toBe("08:00:00");
  });

  it("fetches timespans successfully with data array directly", async () => {
    const mockTimespans = [
      {
        id: 3,
        name: "Evening Session",
        start_time: "18:00:00",
        end_time: "20:00:00",
      },
    ];

    mockApiGet.mockResolvedValueOnce({
      data: mockTimespans,
    });

    const request = createMockRequest("/api/activities/timespans");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string }>;
    }>(response);
    expect(json.data).toHaveLength(1);
    expect(json.data[0]?.id).toBe("3");
  });

  it("returns empty array for unexpected response structure", async () => {
    mockApiGet.mockResolvedValueOnce({
      unexpected: "structure",
    });

    const request = createMockRequest("/api/activities/timespans");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<unknown>;
    }>(response);
    expect(json.data).toEqual([]);
  });

  it("returns empty array on backend error", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Backend error"));

    const request = createMockRequest("/api/activities/timespans");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<unknown>;
    }>(response);
    expect(json.data).toEqual([]);
  });
});
