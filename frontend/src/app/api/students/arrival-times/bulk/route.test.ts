import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { POST } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("@/lib/api-helpers.server", () => ({
  apiGet: vi.fn(),
  apiPost: mockApiPost,
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

describe("POST /api/students/arrival-times/bulk", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students/arrival-times/bulk", {
      method: "POST",
      body: { student_ids: [1, 2], date: "2024-01-15" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches bulk arrival times successfully", async () => {
    const mockArrivalTimes = {
      data: {
        "1": { arrival_time: "08:00", has_exception: false },
        "2": { arrival_time: "08:15", has_exception: true },
      },
    };
    mockApiPost.mockResolvedValueOnce(mockArrivalTimes);

    const request = createMockRequest("/api/students/arrival-times/bulk", {
      method: "POST",
      body: { student_ids: [1, 2], date: "2024-01-15" },
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/students/arrival-times/bulk",
      "test-token",
      { student_ids: [1, 2], date: "2024-01-15" },
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Record<string, unknown>;
    }>(response);
    expect(Object.keys(json.data)).toHaveLength(2);
  });

  it("handles empty student list", async () => {
    mockApiPost.mockResolvedValueOnce({ data: {} });

    const request = createMockRequest("/api/students/arrival-times/bulk", {
      method: "POST",
      body: { student_ids: [], date: "2024-01-15" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<{
      data: Record<string, unknown>;
    }>(response);
    expect(Object.keys(json.data)).toHaveLength(0);
  });
});
