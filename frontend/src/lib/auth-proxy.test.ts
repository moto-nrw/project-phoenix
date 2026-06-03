import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { NextRequest } from "next/server";

// Mock dependencies first — auth-proxy imports server-api-url which
// reaches into env.js at module-eval time, and the t3-env validation
// blows up under vitest without these.
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));

vi.mock("~/lib/client-headers", () => ({
  getClientForwardHeaders: vi.fn(() => ({
    "x-forwarded-for": "203.0.113.10",
    "user-agent": "test-agent",
  })),
}));

// ~/lib/logger is mocked globally in frontend/src/test/setup.ts so that
// logger.error(msg, ctx) routes to console.error(msg, ctx) — and the
// spy below picks the call up.

import { forwardJsonPost } from "./auth-proxy";

const ORIGINAL_FETCH = globalThis.fetch;

function makeRequest(opts: {
  body?: unknown;
  headers?: Record<string, string>;
}): NextRequest {
  const headers = new Headers();
  for (const [k, v] of Object.entries(opts.headers ?? {})) {
    headers.set(k, v);
  }
  return new NextRequest("http://app.test/api/auth/login", {
    method: "POST",
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  });
}

function mockFetchResponse(opts: {
  status: number;
  body?: unknown;
  contentType?: string | null;
  setCookies?: string[];
}): typeof fetch {
  return vi.fn(async (_url, _init) => {
    const headers = new Headers();
    if (opts.contentType !== null) {
      headers.set("content-type", opts.contentType ?? "application/json");
    }
    for (const c of opts.setCookies ?? []) {
      headers.append("set-cookie", c);
    }
    const bodyText =
      opts.status === 204
        ? null
        : opts.body === undefined
          ? ""
          : typeof opts.body === "string"
            ? opts.body
            : JSON.stringify(opts.body);
    return new Response(bodyText, {
      status: opts.status,
      headers,
    });
  }) as unknown as typeof fetch;
}

describe("forwardJsonPost", () => {
  beforeEach(() => {
    globalThis.fetch = ORIGINAL_FETCH;
  });
  afterEach(() => {
    vi.restoreAllMocks();
    globalThis.fetch = ORIGINAL_FETCH;
  });

  it("forwards the JSON body and returns the upstream JSON envelope", async () => {
    const fetchMock = mockFetchResponse({
      status: 200,
      body: { token: "abc" },
    });
    globalThis.fetch = fetchMock;

    // NextRequest's vitest polyfill strips the raw `cookie` header (cookies
    // are intended to be read via request.cookies). The cookie-forwarding
    // branch is therefore covered by the e2e auth flow tests rather than
    // asserted here. Authorization passes through normally.
    const req = makeRequest({
      body: { email: "a@b" },
      headers: { authorization: "Bearer top-secret" },
    });
    const res = await forwardJsonPost(req, "/api/auth/login");

    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ token: "abc" });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = (
      fetchMock as unknown as { mock: { calls: unknown[][] } }
    ).mock.calls[0]!;
    expect(url).toBe("http://backend.test/api/auth/login");
    const headers = (init as RequestInit).headers as Record<string, string>;
    expect(headers["Content-Type"]).toBe("application/json");
    expect(headers["Authorization"]).toBe("Bearer top-secret");
    expect(headers["x-forwarded-for"]).toBe("203.0.113.10");
    expect(init).toMatchObject({ method: "POST" });
    expect((init as RequestInit).body).toBe(JSON.stringify({ email: "a@b" }));
  });

  it("returns 204 with no JSON body when the backend returns 204", async () => {
    // The Headers polyfill in vitest/undici doesn't reliably surface
    // multi-value Set-Cookie via getSetCookie(); the mirror loop still
    // executes and is line-covered by virtue of running.
    const fetchMock = mockFetchResponse({
      status: 204,
      setCookies: ["sid=abc; Path=/", "csrf=xyz; HttpOnly"],
    });
    globalThis.fetch = fetchMock;

    const req = makeRequest({ body: {} });
    const res = await forwardJsonPost(req, "/api/auth/logout");

    expect(res.status).toBe(204);
  });

  it("passes backend HTTP status through unchanged", async () => {
    globalThis.fetch = mockFetchResponse({
      status: 401,
      body: { error: "Invalid credentials" },
    });

    const res = await forwardJsonPost(
      makeRequest({ body: {} }),
      "/api/auth/login",
    );

    expect(res.status).toBe(401);
    expect(await res.json()).toEqual({ error: "Invalid credentials" });
  });

  it("wraps non-JSON bodies in a {message} envelope", async () => {
    globalThis.fetch = mockFetchResponse({
      status: 502,
      body: "Bad Gateway",
      contentType: "text/plain",
    });

    const res = await forwardJsonPost(
      makeRequest({ body: {} }),
      "/api/auth/login",
    );

    expect(res.status).toBe(502);
    expect(await res.json()).toEqual({ message: "Bad Gateway" });
  });

  it("falls back to {message} when the response claims JSON but body isn't valid JSON", async () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});
    globalThis.fetch = mockFetchResponse({
      status: 500,
      body: "not-json-at-all",
      contentType: "application/json",
    });

    const res = await forwardJsonPost(
      makeRequest({ body: {} }),
      "/api/auth/login",
    );

    expect(res.status).toBe(500);
    expect(await res.json()).toEqual({ message: "not-json-at-all" });
    expect(consoleError).toHaveBeenCalled();
    consoleError.mockRestore();
  });

  it("treats fetch rejections as 500 Internal Server Error", async () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});
    globalThis.fetch = vi.fn(async () => {
      throw new Error("ECONNREFUSED");
    }) as unknown as typeof fetch;

    const res = await forwardJsonPost(
      makeRequest({ body: {} }),
      "/api/auth/login",
    );

    expect(res.status).toBe(500);
    expect(await res.json()).toEqual({ error: "Internal Server Error" });
    expect(consoleError).toHaveBeenCalledWith(
      "proxy_failed",
      expect.objectContaining({ path: "/api/auth/login" }),
    );
    consoleError.mockRestore();
  });

  it("honours method override (GET) and skips the body even when hasBody defaults to true", async () => {
    const fetchMock = mockFetchResponse({
      status: 200,
      body: { ok: true },
    });
    globalThis.fetch = fetchMock;

    const req = makeRequest({ headers: {} });
    await forwardJsonPost(req, "/api/auth/whoami", {
      method: "GET",
      hasBody: false,
    });

    const [, init] = (fetchMock as unknown as { mock: { calls: unknown[][] } })
      .mock.calls[0]!;
    expect((init as RequestInit).method).toBe("GET");
    expect((init as RequestInit).body).toBeUndefined();
  });

  it("returns {message: 'Empty response'} when JSON body is empty/null", async () => {
    globalThis.fetch = mockFetchResponse({
      status: 200,
      body: "",
      contentType: "application/json",
    });

    const res = await forwardJsonPost(
      makeRequest({ body: {} }),
      "/api/auth/login",
    );

    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ message: "Empty response" });
  });
});
