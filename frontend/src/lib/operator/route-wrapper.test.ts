import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";

const {
  mockFetch,
  mockGetServerApiUrl,
  mockAuth,
  mockIncomingClientForwardHeaders,
  mockRecordBackendProxyMetric,
} = vi.hoisted(() => ({
  mockIncomingClientForwardHeaders: vi.fn(async () => ({
    "X-Forwarded-For": "203.0.113.4",
    "User-Agent": "operator-browser",
  })),
  mockFetch: vi.fn(),
  mockRecordBackendProxyMetric: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
  mockAuth: vi.fn(),
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

vi.mock("~/lib/backend-proxy-metrics", () => ({
  recordBackendProxyMetric: mockRecordBackendProxyMetric,
}));

vi.mock("~/server/auth/operator", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("operator", {
    operatorAuth: mockAuth,
    uncachedOperatorAuth: mockAuth,
    withOperatorAuth: (handler: unknown) => handler,
  }),
);

// Without a browser request at hand, the identity of the request being
// handled (next/headers outside a request scope here).
vi.mock("~/lib/client-headers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/client-headers.server")>()),
  incomingClientForwardHeaders: mockIncomingClientForwardHeaders,
}));

// Test fixtures below describe backend payloads; materialize actual Fetch
// Responses so the proxy's streaming body path is exercised as in production.
global.fetch = (async (...args: Parameters<typeof fetch>) => {
  const result: unknown = await mockFetch(...args);
  if (result instanceof Response) return result;
  const fixture = result as {
    status: number;
    statusText?: string;
    headers?: Headers;
    json?: () => Promise<unknown>;
    text?: () => Promise<string>;
  };
  const body =
    fixture.status === 204
      ? null
      : fixture.json
        ? JSON.stringify(await fixture.json())
        : fixture.text
          ? await fixture.text()
          : null;
  return new Response(body, {
    status: fixture.status,
    statusText: fixture.statusText,
    headers: fixture.headers,
  });
}) as typeof fetch;

import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import {
  operatorApiGet,
  operatorApiPost,
  createOperatorJsonProxy,
  operatorJsonErrors,
  proxyDelete,
  proxyGet,
  proxyPost,
} from "./route-wrapper.server";

describe("operatorServerFetch", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns unwrapped data from envelope response", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: { id: 1, name: "Test" },
        message: "OK",
      }),
    });

    const result = await operatorApiGet("/api/test", "my-token");

    expect(result).toEqual({ id: 1, name: "Test" });
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/test",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer my-token",
        }) as Record<string, unknown>,
      }),
    );
  });

  it("forwards the operator's client IP and User-Agent", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: {} }),
    });

    await operatorApiGet("/api/test", "my-token");

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/test",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer my-token",
          "X-Forwarded-For": "203.0.113.4",
          "User-Agent": "operator-browser",
        }) as Record<string, unknown>,
      }),
    );
  });

  it("returns undefined for 204 response", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
    });

    const result = await operatorApiGet("/api/test", "my-token");
    expect(result).toBeUndefined();
  });

  it("returns raw JSON for non-envelope response", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ custom: "response" }),
    });

    const result = await operatorApiGet("/api/test", "my-token");
    expect(result).toEqual({ custom: "response" });
  });

  it("throws on 401 error", async () => {
    mockFetch.mockResolvedValue(new Response("Unauthorized", { status: 401 }));

    await expect(
      operatorApiGet("/api/test", "old-token"),
    ).rejects.toMatchObject({ status: 401 });
  });

  it("throws on non-401 error", async () => {
    mockFetch.mockResolvedValue(
      new Response("Internal Server Error", { status: 500 }),
    );

    await expect(operatorApiGet("/api/test", "my-token")).rejects.toMatchObject(
      { status: 500 },
    );

    // Only one fetch call (no retry)
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it("throws a network failure as 503 with its cause and records network_error", async () => {
    const failure = new TypeError("fetch failed");
    mockFetch.mockRejectedValue(failure);

    await expect(operatorApiGet("/api/test", "my-token")).rejects.toMatchObject(
      { status: 503, cause: failure },
    );

    expect(mockRecordBackendProxyMetric).toHaveBeenCalledWith(
      expect.objectContaining({ outcome: "network_error", status: 0 }),
    );
  });

  it("sends POST body correctly", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: { created: true },
      }),
    });

    const result = await operatorApiPost("/api/test", "my-token", {
      name: "test",
    });

    expect(result).toEqual({ created: true });
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/test",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ name: "test" }),
      }),
    );
  });

  it("sends DELETE request correctly via operatorApiDelete", async () => {
    const { operatorApiDelete } = await import("./route-wrapper.server");

    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
    });

    const result = await operatorApiDelete("/api/test/1", "my-token");

    expect(result).toBeUndefined();
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/test/1",
      expect.objectContaining({
        method: "DELETE",
      }),
    );
  });
});

describe("operator proxy factories", () => {
  const mockContext: RouteContext = { params: Promise.resolve({ id: "42" }) };
  const mockSession = { user: { token: "operator-token" } };

  function createProxyRequest(
    path: string,
    options: { method?: string; body?: unknown } = {},
  ): NextRequest {
    const init: { method: string; body?: string; headers?: HeadersInit } = {
      method: options.method ?? "GET",
    };
    if (options.body !== undefined) {
      init.body = JSON.stringify(options.body);
      init.headers = { "Content-Type": "application/json" };
    }
    return new NextRequest(`http://localhost:3000${path}`, init);
  }

  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(mockSession);
  });

  it("uses operator auth, forwards query strings, and wraps already-unwrapped GET data once", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: { id: 42, name: "Operator Item" },
      }),
    });

    const handler = proxyGet(
      (params) => `/operator/items/${String(params.id)}`,
    );
    const response = await handler(
      createProxyRequest("/api/operator/items/42?include=stats"),
      mockContext,
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/items/42?include=stats",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer operator-token",
        }) as Record<string, unknown>,
      }),
    );
    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual({
      success: true,
      message: "Success",
      data: { id: 42, name: "Operator Item" },
    });
  });

  it("forwards POST bodies through the operator proxy", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: { created: true },
      }),
    });

    const handler = proxyPost("/operator/items");
    const response = await handler(
      createProxyRequest("/api/operator/items", {
        method: "POST",
        body: { name: "New" },
      }),
      { params: Promise.resolve({}) },
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/items",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ name: "New" }),
      }),
    );
    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toMatchObject({
      data: { created: true },
    });
  });

  it("resolves params for DELETE and returns 204", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
    });

    const handler = proxyDelete(
      (params) => `/operator/items/${String(params.id)}`,
    );
    const response = await handler(
      createProxyRequest("/api/operator/items/42", { method: "DELETE" }),
      mockContext,
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/items/42",
      expect.objectContaining({ method: "DELETE" }),
    );
    expect(response.status).toBe(204);
    await expect(response.text()).resolves.toBe("");
  });
});

describe("operator JSON proxies", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({ user: { token: "operator-token" } });
  });

  const post = (body: string) =>
    new NextRequest("http://localhost:3000/api/operator/invitations", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "x-forwarded-for": "198.51.100.9",
        "user-agent": "operator-browser",
      },
      body,
    });

  it("forwards the body verbatim with the operator token and client identity", async () => {
    mockFetch.mockResolvedValue(
      Response.json({ status: "success", data: { id: 3 } }, { status: 201 }),
    );

    const response = await createOperatorJsonProxy({
      method: "POST",
      path: "/operator/invitations",
    })(post('{"email":"a@b.de"}'), { params: Promise.resolve({}) });

    expect(response.status).toBe(201);
    expect(await response.json()).toEqual({
      status: "success",
      data: { id: 3 },
    });
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/invitations",
      expect.objectContaining({
        method: "POST",
        body: '{"email":"a@b.de"}',
        headers: expect.objectContaining({
          Authorization: "Bearer operator-token",
          "X-Forwarded-For": "198.51.100.9",
          "User-Agent": "operator-browser",
        }) as Record<string, unknown>,
      }),
    );
  });

  it("answers an unreadable body with 400 general.input before any call", async () => {
    const response = await createPublicJsonProxy({
      method: "POST",
      path: "/operator/auth/email-confirm",
      ...operatorJsonErrors,
    })(post("not json{"));

    expect(response.status).toBe(400);
    expect(await response.json()).toMatchObject({ code: "general.input" });
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("answers an unreachable backend with 503 general.unavailable", async () => {
    mockFetch.mockRejectedValue(new TypeError("fetch failed"));

    const response = await createPublicJsonProxy({
      method: "POST",
      path: "/operator/auth/email-confirm",
      ...operatorJsonErrors,
    })(post("{}"));

    expect(response.status).toBe(503);
    expect(await response.json()).toMatchObject({
      code: "general.unavailable",
    });
  });
});
