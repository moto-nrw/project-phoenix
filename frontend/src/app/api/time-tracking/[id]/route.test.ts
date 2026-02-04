import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { PUT } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiPut } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPut: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: vi.fn(),
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

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("PUT /api/time-tracking/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/time-tracking/42", {
      method: "PUT",
      body: { status: "home_office" },
    });
    const response = await PUT(request, createMockContext({ id: "42" }));

    expect(response.status).toBe(401);
  });

  it("updates work session successfully", async () => {
    const mockUpdatedSession = {
      id: 42,
      status: "home_office",
      notes: "Working from home",
    };
    mockApiPut.mockResolvedValueOnce({ data: mockUpdatedSession });

    const request = createMockRequest("/api/time-tracking/42", {
      method: "PUT",
      body: {
        status: "home_office",
        notes: "Working from home",
      },
    });
    const response = await PUT(request, createMockContext({ id: "42" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/time-tracking/42",
      "test-token",
      {
        check_in_time: undefined,
        check_out_time: undefined,
        break_minutes: undefined,
        status: "home_office",
        notes: "Working from home",
      },
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedSession>>(response);
    expect(json.data).toEqual(mockUpdatedSession);
  });

  it("converts camelCase to snake_case for backend", async () => {
    mockApiPut.mockResolvedValueOnce({ data: {} });

    const request = createMockRequest("/api/time-tracking/42", {
      method: "PUT",
      body: {
        checkInTime: "2024-01-15T08:00:00Z",
        checkOutTime: "2024-01-15T17:00:00Z",
        breakMinutes: 30,
      },
    });
    await PUT(request, createMockContext({ id: "42" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/time-tracking/42",
      "test-token",
      {
        check_in_time: "2024-01-15T08:00:00Z",
        check_out_time: "2024-01-15T17:00:00Z",
        break_minutes: 30,
        status: undefined,
        notes: undefined,
      },
    );
  });

  it("converts break IDs from string to int", async () => {
    mockApiPut.mockResolvedValueOnce({ data: {} });

    const request = createMockRequest("/api/time-tracking/42", {
      method: "PUT",
      body: {
        breaks: [
          { id: "1", durationMinutes: 15 },
          { id: "2", durationMinutes: 30 },
        ],
      },
    });
    await PUT(request, createMockContext({ id: "42" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/time-tracking/42",
      "test-token",
      {
        check_in_time: undefined,
        check_out_time: undefined,
        break_minutes: undefined,
        status: undefined,
        notes: undefined,
        breaks: [
          { id: 1, duration_minutes: 15 },
          { id: 2, duration_minutes: 30 },
        ],
      },
    );
  });
});
