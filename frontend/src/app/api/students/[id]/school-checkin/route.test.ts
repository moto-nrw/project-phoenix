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
      : message.includes("(403)")
        ? 403
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
  const init: { method: string; body?: string; headers?: HeadersInit } = {
    method: options.method ?? "GET",
  };
  if (options.body) {
    init.body = JSON.stringify(options.body);
    init.headers = { "Content-Type": "application/json" };
  }
  return new NextRequest(url, init);
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

describe("POST /api/students/[id]/school-checkin", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);
    const request = createMockRequest("/api/students/42/school-checkin", {
      method: "POST",
      body: { action: "in" },
    });
    const response = await POST(request, createMockContext({ id: "42" }));
    expect(response.status).toBe(401);
  });

  it("proxies the body + token to the backend and returns the mapped result", async () => {
    mockApiPost.mockResolvedValueOnce({
      data: {
        student_id: 42,
        status: "checked_in",
        location: "Anwesend",
        changed: true,
      },
    });

    const request = createMockRequest("/api/students/42/school-checkin", {
      method: "POST",
      body: { action: "in" },
    });
    const response = await POST(request, createMockContext({ id: "42" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/students/42/school-checkin",
      "test-token",
      { action: "in" },
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      data: { status: string; location: string; changed: boolean };
    }>(response);
    expect(json.data.status).toBe("checked_in");
    expect(json.data.location).toBe("Anwesend");
    expect(json.data.changed).toBe(true);
  });

  it("propagates backend 403 unchanged", async () => {
    mockApiPost.mockRejectedValueOnce(
      new Error("API error (403): not a supervisor"),
    );
    const request = createMockRequest("/api/students/42/school-checkin", {
      method: "POST",
      body: { action: "in" },
    });
    const response = await POST(request, createMockContext({ id: "42" }));
    expect(response.status).toBe(403);
  });

  it("rejects a missing id param", async () => {
    const request = createMockRequest("/api/students//school-checkin", {
      method: "POST",
      body: { action: "in" },
    });
    // Empty id → the route's "Student ID is required" throw surfaces as 500.
    const response = await POST(request, createMockContext({ id: "" }));
    expect(response.status).toBe(500);
  });
});
