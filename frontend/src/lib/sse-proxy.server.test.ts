import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import type { Logger } from "~/lib/logger";
import { proxySSEStream } from "./sse-proxy.server";

const { captureException } = vi.hoisted(() => ({ captureException: vi.fn() }));
vi.mock("@sentry/nextjs", () => ({ captureException }));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://localhost:8080",
}));

const originalFetch = global.fetch;
beforeEach(() => vi.clearAllMocks());
afterEach(() => {
  global.fetch = originalFetch;
});

it("forwards an SSE backend error without inventing a connection message", async () => {
  const body =
    '{"code":"stream_conflict","details":{"id":"42"},"errors":[],"instance":"/api/sse/events"}';
  const mockFetch = vi.fn().mockResolvedValue(
    new Response(body, {
      status: 409,
      headers: { "Content-Type": "application/problem+json" },
    }),
  );
  global.fetch = mockFetch as typeof fetch;
  const logger = { error: vi.fn(), debug: vi.fn() } as unknown as Logger;

  const response = await proxySSEStream(
    new NextRequest("http://localhost:3000/api/sse/events"),
    { token: "test-token", upstreamPath: "/api/sse/events", logger },
  );

  expect(response.status).toBe(409);
  expect(response.headers.get("Content-Type")).toBe("application/problem+json");
  expect(await response.text()).toBe(body);
  expect(mockFetch).toHaveBeenCalledWith(
    "http://localhost:8080/api/sse/events",
    expect.objectContaining({
      headers: expect.objectContaining({ Authorization: "Bearer test-token" }),
    }),
  );
  expect(captureException).not.toHaveBeenCalled();
});

it("forwards an SSE backend 5xx without a BFF event", async () => {
  global.fetch = vi
    .fn()
    .mockResolvedValue(
      new Response("backend down", { status: 503 }),
    ) as typeof fetch;
  const logger = { error: vi.fn(), debug: vi.fn() } as unknown as Logger;

  const response = await proxySSEStream(
    new NextRequest("http://localhost:3000/api/sse/events"),
    { token: "test-token", upstreamPath: "/api/sse/events", logger },
  );

  expect(response.status).toBe(503);
  expect(await response.text()).toBe("backend down");
  expect(captureException).not.toHaveBeenCalled();
});

it("reports an unreachable SSE backend exactly once with the request ID", async () => {
  const failure = new TypeError("fetch failed");
  global.fetch = vi.fn().mockRejectedValue(failure) as typeof fetch;
  const logger = { error: vi.fn(), debug: vi.fn() } as unknown as Logger;
  const request = new NextRequest("http://localhost:3000/api/sse/events", {
    headers: { "X-Request-ID": "0b6f3f4e-5c1d-4a52-9d57-2d3c1b5e8f10" },
  });

  const response = await proxySSEStream(request, {
    token: "test-token",
    upstreamPath: "/api/sse/events",
    logger,
  });

  expect(response.status).toBe(500);
  expect(captureException).toHaveBeenCalledExactlyOnceWith(failure, {
    tags: { request_id: "0b6f3f4e-5c1d-4a52-9d57-2d3c1b5e8f10" },
  });
});

it("does not report an aborted SSE request", async () => {
  global.fetch = vi
    .fn()
    .mockRejectedValue(
      new DOMException("aborted", "AbortError"),
    ) as typeof fetch;
  const logger = { error: vi.fn(), debug: vi.fn() } as unknown as Logger;

  const response = await proxySSEStream(
    new NextRequest("http://localhost:3000/api/sse/events"),
    { token: "test-token", upstreamPath: "/api/sse/events", logger },
  );

  expect(response.status).toBe(204);
  expect(captureException).not.toHaveBeenCalled();
});
