import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, PUT } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiGet, mockApiPut } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPut: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("@/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: mockApiPut,
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

function createMockRequest(
  path: string,
  options: { method?: string; body?: unknown } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const requestInit: { method: string; body?: string; headers?: HeadersInit } =
    {
      method: options.method ?? "GET",
    };
  if (options.body) {
    requestInit.body = JSON.stringify(options.body);
    requestInit.headers = { "Content-Type": "application/json" };
  }
  return new NextRequest(url, requestInit);
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

describe("GET /api/students/[id]/arrival-schedules", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students/123/arrival-schedules");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("fetches arrival schedules from backend", async () => {
    const mockSchedules = {
      data: {
        schedules: [
          { day: 1, arrival_time: "08:00" },
          { day: 2, arrival_time: "08:30" },
        ],
        exceptions: [],
        notes: [],
      },
    };
    mockApiGet.mockResolvedValueOnce(mockSchedules);

    const request = createMockRequest("/api/students/123/arrival-schedules");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/students/123/arrival-schedules",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: { schedules: unknown[] };
    }>(response);
    expect(json.data.schedules).toHaveLength(2);
  });

  it("handles backend errors on GET", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Server error"));

    const request = createMockRequest("/api/students/123/arrival-schedules");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(500);
  });
});

describe("PUT /api/students/[id]/arrival-schedules", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students/123/arrival-schedules", {
      method: "PUT",
      body: { schedules: [{ day: 1, arrival_time: "08:00" }] },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("updates arrival schedules successfully", async () => {
    const mockUpdated = {
      data: {
        schedules: [{ day: 1, arrival_time: "08:00" }],
      },
    };
    mockApiPut.mockResolvedValueOnce(mockUpdated);

    const request = createMockRequest("/api/students/123/arrival-schedules", {
      method: "PUT",
      body: { schedules: [{ day: 1, arrival_time: "08:00" }] },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/students/123/arrival-schedules",
      "test-token",
      { schedules: [{ day: 1, arrival_time: "08:00" }] },
    );
    expect(response.status).toBe(200);
  });

  it("handles backend errors on PUT", async () => {
    mockApiPut.mockRejectedValueOnce(new Error("Validation failed"));

    const request = createMockRequest("/api/students/123/arrival-schedules", {
      method: "PUT",
      body: { schedules: [] },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(500);
  });
});
