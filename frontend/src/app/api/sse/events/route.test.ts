import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { GET } from "./route";
import { auth } from "~/server/auth";

// ============================================================================
// Mocks
// ============================================================================

const mockFetch = vi.fn();

// Mock global fetch
vi.stubGlobal("fetch", mockFetch);

// Note: auth() and getCookieHeader() are globally mocked in setup.ts

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(path: string, search = ""): NextRequest {
  const url = new URL(path + search, "http://localhost:3000");
  return new NextRequest(url);
}

const TEST_COOKIE_HEADER = "better-auth.session_token=test-session-token";

function createReadableStream(chunks: string[]): ReadableStream<Uint8Array> {
  let index = 0;
  return new ReadableStream({
    pull(controller) {
      if (index < chunks.length) {
        controller.enqueue(new TextEncoder().encode(chunks[index]));
        index++;
      } else {
        controller.close();
      }
    },
  });
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/sse/events", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns 401 when not authenticated", async () => {
    // Mock auth to return null
    vi.mocked(auth).mockResolvedValueOnce(null);

    const request = createMockRequest("/api/sse/events");
    const response = await GET(request);

    expect(response.status).toBe(401);
    expect(await response.text()).toBe("Unauthorized");
  });

  it("returns 401 when user is undefined in session", async () => {
    vi.mocked(auth).mockResolvedValueOnce({ user: undefined } as never);

    const request = createMockRequest("/api/sse/events");
    const response = await GET(request);

    expect(response.status).toBe(401);
  });

  it("proxies SSE stream from backend on success", async () => {
    const sseData = 'data: {"type":"heartbeat"}\n\n';
    const mockBody = createReadableStream([sseData]);

    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      body: mockBody,
      headers: new Headers({
        "Content-Type": "text/event-stream",
      }),
    });

    const request = createMockRequest("/api/sse/events");
    const response = await GET(request);

    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Type")).toBe("text/event-stream");
    expect(response.headers.get("Cache-Control")).toBe("no-cache");
    expect(response.headers.get("Connection")).toBe("keep-alive");
    expect(response.headers.get("X-Accel-Buffering")).toBe("no");

    // Verify fetch was called with correct params
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/sse/events",
      {
        headers: {
          Cookie: TEST_COOKIE_HEADER,
          Accept: "text/event-stream",
        },
        cache: "no-store",
      },
    );
  });

  it("forwards query parameters to backend", async () => {
    const mockBody = createReadableStream(["data: test\n\n"]);

    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      body: mockBody,
    });

    const request = createMockRequest("/api/sse/events", "?cacheBuster=123");
    await GET(request);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/sse/events?cacheBuster=123",
      expect.objectContaining({
        headers: expect.objectContaining({
          Cookie: TEST_COOKIE_HEADER,
        }),
      }),
    );
  });

  it("returns backend error status when backend fails", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 403,
      text: vi.fn().mockResolvedValue("Forbidden"),
    });

    const request = createMockRequest("/api/sse/events");
    const response = await GET(request);

    expect(response.status).toBe(403);
    expect(await response.text()).toBe("Forbidden");
  });

  it("returns 502 when backend response has no body", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      body: null,
    });

    const request = createMockRequest("/api/sse/events");
    const response = await GET(request);

    expect(response.status).toBe(502);
    expect(await response.text()).toBe("No response body from backend");
  });

  it("returns 500 when fetch throws error", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/sse/events");
    const response = await GET(request);

    expect(response.status).toBe(500);
    expect(await response.text()).toBe("Internal server error");
  });

  it("returns default error message when backend text fails", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      text: vi.fn().mockRejectedValue(new Error("Failed to read")),
    });

    const request = createMockRequest("/api/sse/events");
    const response = await GET(request);

    expect(response.status).toBe(500);
    expect(await response.text()).toBe("SSE connection failed");
  });
});
