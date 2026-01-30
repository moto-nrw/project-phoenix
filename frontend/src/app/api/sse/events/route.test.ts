import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

// Mock global fetch
global.fetch = mockFetch;

// Mock env
vi.mock("~/env", () => ({
  env: {
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

function createMockRequest(path: string): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  return new NextRequest(url);
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

describe("GET /api/sse/events", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(console, "error").mockImplementation(() => {
      /* noop */
    });
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/sse/events");
    const response = await GET(request);

    expect(response.status).toBe(401);
    const text = await response.text();
    expect(text).toBe("Unauthorized");
  });

  it("returns 401 when session has no token", async () => {
    mockAuth.mockResolvedValueOnce({
      user: { id: "1", name: "Test" },
      expires: "2099-01-01",
    });

    const request = createMockRequest("/api/sse/events");
    const response = await GET(request);

    expect(response.status).toBe(401);
  });

  it("proxies SSE stream from backend", async () => {
    const mockStream = new ReadableStream({
      start(controller) {
        controller.enqueue(new TextEncoder().encode("data: test\n\n"));
        controller.close();
      },
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      body: mockStream,
    });

    const request = createMockRequest("/api/sse/events");
    const response = await GET(request);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/sse/events",
      expect.objectContaining({
        headers: {
          Authorization: "Bearer test-token",
          Accept: "text/event-stream",
        },
        cache: "no-store",
      }),
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Type")).toBe("text/event-stream");
    expect(response.headers.get("Cache-Control")).toBe("no-cache");
    expect(response.headers.get("Connection")).toBe("keep-alive");
    expect(response.headers.get("X-Accel-Buffering")).toBe("no");
  });

  it("preserves query parameters", async () => {
    const mockStream = new ReadableStream({
      start(controller) {
        controller.close();
      },
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      body: mockStream,
    });

    const request = createMockRequest("/api/sse/events?cache=123");
    await GET(request);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/sse/events?cache=123",
      expect.any(Object),
    );
  });

  it("returns backend error status", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 403,
      text: async () => "Forbidden",
    });

    const request = createMockRequest("/api/sse/events");
    const response = await GET(request);

    expect(response.status).toBe(403);
    const text = await response.text();
    expect(text).toBe("Forbidden");
  });

  it("handles backend error without body", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      text: async () => {
        throw new Error("Cannot read body");
      },
    });

    const request = createMockRequest("/api/sse/events");
    const response = await GET(request);

    expect(response.status).toBe(500);
    const text = await response.text();
    expect(text).toBe("SSE connection failed");
  });

  it("returns 502 when backend has no body", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      body: null,
    });

    const request = createMockRequest("/api/sse/events");
    const response = await GET(request);

    expect(response.status).toBe(502);
    const text = await response.text();
    expect(text).toBe("No response body from backend");
  });

  it("handles fetch exceptions", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/sse/events");
    const response = await GET(request);

    expect(response.status).toBe(500);
    const text = await response.text();
    expect(text).toBe("Internal server error");
  });
});
