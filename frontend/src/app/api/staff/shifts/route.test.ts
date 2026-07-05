import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, POST } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiGet, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", () => ({
  apiGet: mockApiGet,
  apiPost: mockApiPost,
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
  handleApiError: vi.fn(
    () => new Response(JSON.stringify({ error: "err" }), { status: 500 }),
  ),
}));

function createMockRequest(
  path: string,
  options: { method?: string; body?: unknown } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const requestInit: { method: string; body?: string; headers?: HeadersInit } =
    { method: options.method ?? "GET" };
  if (options.body) {
    requestInit.body = JSON.stringify(options.body);
    requestInit.headers = { "Content-Type": "application/json" };
  }
  return new NextRequest(url, requestInit);
}

function createMockContext() {
  return { params: Promise.resolve({}) };
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

describe("GET /api/staff/shifts", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await GET(
      createMockRequest("/api/staff/shifts"),
      createMockContext(),
    );
    expect(response.status).toBe(401);
  });

  it("proxies to the backend staff-shifts endpoint with the query string", async () => {
    mockApiGet.mockResolvedValueOnce({ data: [] });

    const response = await GET(
      createMockRequest("/api/staff/shifts?from=2026-07-06&to=2026-07-10"),
      createMockContext(),
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/staff-shifts?from=2026-07-06&to=2026-07-10",
      "test-token",
    );
    expect(response.status).toBe(200);
  });
});

describe("POST /api/staff/shifts", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("forwards the shift body to the backend", async () => {
    const body = {
      staff_id: 7,
      date: "2026-07-06",
      start_time: "08:00",
      end_time: "16:00",
      break_minutes: 30,
    };
    mockApiPost.mockResolvedValueOnce({ data: { id: 42, ...body } });

    const response = await POST(
      createMockRequest("/api/staff/shifts", { method: "POST", body }),
      createMockContext(),
    );

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/staff-shifts",
      "test-token",
      body,
    );
    expect(response.status).toBe(200);
  });
});
