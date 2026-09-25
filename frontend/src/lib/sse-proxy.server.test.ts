import { afterEach, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import type { Logger } from "~/lib/logger";
import { proxySSEStream } from "./sse-proxy.server";

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://localhost:8080",
}));

const originalFetch = global.fetch;
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
});
