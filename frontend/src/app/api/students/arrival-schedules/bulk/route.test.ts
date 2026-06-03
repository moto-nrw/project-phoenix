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

describe("POST /api/students/arrival-schedules/bulk", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students/arrival-schedules/bulk", {
      method: "POST",
      body: { school_class_id: 5, schedules: [] },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("bulk upserts arrival schedules successfully", async () => {
    const mockResponse = {
      data: {
        updated: 10,
        created: 5,
      },
    };
    mockApiPost.mockResolvedValueOnce(mockResponse);

    const body = {
      school_class_id: 5,
      schedules: [{ day: 1, arrival_time: "08:00" }],
    };
    const request = createMockRequest("/api/students/arrival-schedules/bulk", {
      method: "POST",
      body,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/students/arrival-schedules/bulk",
      "test-token",
      body,
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: { updated: number; created: number };
    }>(response);
    expect(json.data.updated).toBe(10);
    expect(json.data.created).toBe(5);
  });

  it("handles backend errors", async () => {
    mockApiPost.mockRejectedValueOnce(new Error("Backend failure"));

    const request = createMockRequest("/api/students/arrival-schedules/bulk", {
      method: "POST",
      body: { school_class_id: 5, schedules: [] },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Backend failure");
  });
});
