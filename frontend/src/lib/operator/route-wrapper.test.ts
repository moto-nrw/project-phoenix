import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";

const {
  mockFetch,
  mockGetServerApiUrl,
  mockAuth,
  mockGetClientForwardHeaders,
} = vi.hoisted(() => ({
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
  mockAuth: vi.fn(),
  mockGetClientForwardHeaders: vi.fn(() => ({
    "X-Forwarded-For": "127.0.0.1",
    "X-Real-IP": "127.0.0.1",
    "User-Agent": "test-agent",
  })),
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: mockAuth,
}));

vi.mock("~/lib/client-headers.server", () => ({
  getClientForwardHeaders: mockGetClientForwardHeaders,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import {
  operatorApiGet,
  operatorApiPost,
  createOperatorProxyPostHandler,
  createOperatorPublicProxyPostHandler,
  createOperatorProxyMethodHandler,
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
    mockFetch.mockResolvedValue({
      ok: false,
      status: 401,
      text: async () => "Unauthorized",
    });

    await expect(operatorApiGet("/api/test", "old-token")).rejects.toThrow(
      "API error (401)",
    );
  });

  it("throws on non-401 error", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      text: async () => "Internal Server Error",
    });

    await expect(operatorApiGet("/api/test", "my-token")).rejects.toThrow(
      "API error (500)",
    );

    // Only one fetch call (no retry)
    expect(mockFetch).toHaveBeenCalledTimes(1);
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

  it("sends PUT request correctly via operatorApiPut", async () => {
    const { operatorApiPut } = await import("./route-wrapper.server");

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: { updated: true },
      }),
    });

    const result = await operatorApiPut("/api/test", "my-token", {
      name: "updated",
    });

    expect(result).toEqual({ updated: true });
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/test",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ name: "updated" }),
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

  it("retries proxy requests with a refreshed operator token after backend 401", async () => {
    mockAuth
      .mockResolvedValueOnce({ user: { token: "old-token" } })
      .mockResolvedValueOnce({ user: { token: "new-token" } });
    mockFetch
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () => "Unauthorized",
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          status: "success",
          data: { refreshed: true },
        }),
      });

    const handler = proxyGet("/operator/items");
    const response = await handler(createProxyRequest("/api/operator/items"), {
      params: Promise.resolve({}),
    });

    expect(mockFetch).toHaveBeenNthCalledWith(
      1,
      "http://localhost:8080/operator/items",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer old-token",
        }) as Record<string, unknown>,
      }),
    );
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      "http://localhost:8080/operator/items",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer new-token",
        }) as Record<string, unknown>,
      }),
    );
    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toMatchObject({
      data: { refreshed: true },
    });
  });
});

describe("createOperatorProxyPostHandler", () => {
  const mockContext: RouteContext = { params: Promise.resolve({}) };

  function createMockRequest(body: unknown): NextRequest {
    return new NextRequest(
      "http://localhost:3000/api/operator/profile/email-change",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      },
    );
  }

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const handler = createOperatorProxyPostHandler(
      "/operator/profile/email-change",
    );
    const request = createMockRequest({ email: "new@test.com" });
    const response = await handler(request, mockContext);

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string };
    expect(json.error).toBe("Unauthorized");
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("proxies successful JSON response with status", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({ message: "E-Mail-Änderung eingeleitet" }),
    });

    const handler = createOperatorProxyPostHandler(
      "/operator/profile/email-change",
    );
    const request = createMockRequest({
      email: "new@test.com",
      password: "pass",
    });
    const response = await handler(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("E-Mail-Änderung eingeleitet");
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/profile/email-change",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          Authorization: "Bearer valid-token",
          "Content-Type": "application/json",
          "X-Forwarded-For": "127.0.0.1",
        }) as Record<string, unknown>,
      }),
    );
  });

  it("returns German message for 429 non-JSON response", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 429,
      headers: new Headers({ "content-type": "text/plain" }),
      text: async () => "Too Many Requests",
    });

    const handler = createOperatorProxyPostHandler(
      "/operator/profile/email-change",
    );
    const request = createMockRequest({ email: "new@test.com" });
    const response = await handler(request, mockContext);

    expect(response.status).toBe(429);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe(
      "Zu viele Anfragen. Bitte versuchen Sie es später erneut.",
    );
  });

  it("returns text body for non-JSON, non-429 response", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 502,
      headers: new Headers({ "content-type": "text/html" }),
      text: async () => "Bad Gateway",
    });

    const handler = createOperatorProxyPostHandler(
      "/operator/profile/email-change",
    );
    const request = createMockRequest({ email: "new@test.com" });
    const response = await handler(request, mockContext);

    expect(response.status).toBe(502);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Bad Gateway");
  });

  it("retries on 401 with refreshed token", async () => {
    mockAuth
      .mockResolvedValueOnce({ user: { token: "old-token" } })
      .mockResolvedValueOnce({ user: { token: "new-token" } });

    mockFetch
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        headers: new Headers({ "content-type": "application/json" }),
        json: async () => ({ error: "Unauthorized" }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        headers: new Headers({ "content-type": "application/json" }),
        json: async () => ({ message: "Success after retry" }),
      });

    const handler = createOperatorProxyPostHandler(
      "/operator/profile/email-change",
    );
    const request = createMockRequest({ email: "new@test.com" });
    const response = await handler(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Success after retry");
    expect(mockFetch).toHaveBeenCalledTimes(2);
  });

  it("returns original 401 response when token refresh yields same token", async () => {
    mockAuth.mockResolvedValue({ user: { token: "same-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 401,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({ error: "Unauthorized" }),
    });

    const handler = createOperatorProxyPostHandler(
      "/operator/profile/email-change",
    );
    const request = createMockRequest({ email: "new@test.com" });
    const response = await handler(request, mockContext);

    expect(response.status).toBe(401);
  });

  it("proxies backend error JSON with original status code", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 400,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({ message: "Ungültige E-Mail-Adresse" }),
    });

    const handler = createOperatorProxyPostHandler(
      "/operator/profile/email-change",
    );
    const request = createMockRequest({ email: "bad" });
    const response = await handler(request, mockContext);

    expect(response.status).toBe(400);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Ungültige E-Mail-Adresse");
  });

  it("returns statusText when text body is empty for non-JSON response", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 503,
      statusText: "Service Unavailable",
      headers: new Headers({ "content-type": "text/plain" }),
      text: async () => "",
    });

    const handler = createOperatorProxyPostHandler(
      "/operator/profile/email-change",
    );
    const request = createMockRequest({ email: "new@test.com" });
    const response = await handler(request, mockContext);

    expect(response.status).toBe(503);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Service Unavailable");
  });

  it("handles fetch error gracefully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockRejectedValue(new Error("Network error"));

    const handler = createOperatorProxyPostHandler(
      "/operator/profile/email-change",
    );
    const request = createMockRequest({ email: "new@test.com" });
    const response = await handler(request, mockContext);

    expect(response.status).toBe(500);
  });

  it("returns 400 for invalid JSON body", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const handler = createOperatorProxyPostHandler(
      "/operator/profile/email-change",
    );
    const request = new NextRequest(
      "http://localhost:3000/api/operator/profile/email-change",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: "not valid json{{{",
      },
    );
    const response = await handler(request, mockContext);

    expect(response.status).toBe(400);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Ungültige Anfrage");
    expect(mockFetch).not.toHaveBeenCalled();
  });
});

// =============================================================================
// createOperatorPublicProxyPostHandler
// =============================================================================
//
// Uses a synthetic endpoint ("/test/public"). Each real invitation route that
// calls this helper ({validate, accept}) also has a minimal smoke test that
// pins its actual endpoint string — see
// app/api/operator/auth/invitations/{validate,accept}/route.test.ts.

describe("createOperatorPublicProxyPostHandler", () => {
  const handler = createOperatorPublicProxyPostHandler("/test/public");

  function makePublicRequest(body: unknown): NextRequest {
    return new NextRequest("http://localhost:3000/api/operator/public/test", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  }

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns 400 with German 'Ungültige Anfrage' on invalid JSON body", async () => {
    const request = new NextRequest(
      "http://localhost:3000/api/operator/public/test",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: "not valid json{{{",
      },
    );

    const response = await handler(request);

    expect(response.status).toBe(400);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Ungültige Anfrage");
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("proxies JSON body to backend with correct method, Content-Type and URL", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({ message: "ok" }),
    });

    const body = { token: "abc", displayName: "Test" };
    await handler(makePublicRequest(body));

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/test/public",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          "Content-Type": "application/json",
        }) as Record<string, unknown>,
        body: JSON.stringify(body),
      }),
    );
  });

  it("forwards client headers (X-Forwarded-For, User-Agent) to backend", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({}),
    });

    await handler(makePublicRequest({ token: "abc" }));

    expect(mockFetch).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        headers: expect.objectContaining({
          "X-Forwarded-For": "127.0.0.1",
          "User-Agent": "test-agent",
        }) as Record<string, unknown>,
      }),
    );
  });

  it("forwards successful JSON response verbatim with status preserved", async () => {
    const backendBody = { email: "test@example.com", expiresAt: "2026-05-01" };
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => backendBody,
    });

    const response = await handler(makePublicRequest({ token: "abc" }));

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual(backendBody);
  });

  it("forwards backend JSON error body with original status", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 400,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({ message: "Passwort zu schwach" }),
    });

    const response = await handler(makePublicRequest({ token: "abc" }));

    expect(response.status).toBe(400);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Passwort zu schwach");
  });

  it("returns German rate-limit message for 429 non-JSON response", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 429,
      headers: new Headers({ "content-type": "text/plain" }),
      text: async () => "Too Many Requests",
    });

    const response = await handler(makePublicRequest({ token: "abc" }));

    expect(response.status).toBe(429);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe(
      "Zu viele Anfragen. Bitte versuchen Sie es später erneut.",
    );
  });

  it("forwards backend 429 JSON body verbatim (preserves backend-specific messaging)", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 429,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({
        status: "Too Many Requests",
        message: "Zu viele Einladungen. Bitte warte eine Stunde.",
      }),
    });

    const response = await handler(makePublicRequest({ token: "abc" }));

    expect(response.status).toBe(429);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Zu viele Einladungen. Bitte warte eine Stunde.");
  });

  it("wraps non-JSON text body in { message } envelope", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 502,
      headers: new Headers({ "content-type": "text/html" }),
      text: async () => "Bad Gateway",
    });

    const response = await handler(makePublicRequest({ token: "abc" }));

    expect(response.status).toBe(502);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Bad Gateway");
  });

  it("falls back to statusText when non-JSON body is empty", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 503,
      statusText: "Service Unavailable",
      headers: new Headers({ "content-type": "text/plain" }),
      text: async () => "",
    });

    const response = await handler(makePublicRequest({ token: "abc" }));

    expect(response.status).toBe(503);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Service Unavailable");
  });

  it("returns 500 with generic German message on fetch error", async () => {
    mockFetch.mockRejectedValue(new Error("Network error"));

    const response = await handler(makePublicRequest({ token: "abc" }));

    expect(response.status).toBe(500);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Ein interner Fehler ist aufgetreten");
  });
});

// =============================================================================
// createOperatorProxyMethodHandler
// =============================================================================
//
// Uses synthetic dynamic endpoints and both DELETE + POST variants to exercise
// the method-agnostic behavior. Each real invitation route that calls this
// helper ({[id], [id]/resend}) has a minimal smoke test that pins its actual
// endpoint + method — see
// app/api/operator/invitations/[id]/{route,resend/route}.test.ts.

describe("createOperatorProxyMethodHandler", () => {
  const deleteHandler = createOperatorProxyMethodHandler(
    "DELETE",
    (params) => `/test/${String(params.id)}`,
  );
  const postHandler = createOperatorProxyMethodHandler(
    "POST",
    (params) => `/test/${String(params.id)}/action`,
  );

  const mockSession = { user: { token: "access-token-123" } };

  function makeAuthRequest(method = "DELETE"): NextRequest {
    return new NextRequest("http://localhost:3000/api/operator/test/42", {
      method,
    });
  }

  function makeContext(id = "42"): RouteContext {
    return { params: Promise.resolve({ id }) };
  }

  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(mockSession);
  });

  it("returns 401 TOKEN_EXPIRED envelope when no session", async () => {
    mockAuth.mockResolvedValue(null);

    const response = await deleteHandler(makeAuthRequest(), makeContext());

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string; code?: string };
    expect(json.code).toBe("TOKEN_EXPIRED");
    expect(json.error).toBe("Unauthorized");
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("returns 401 TOKEN_EXPIRED envelope when session token is null", async () => {
    mockAuth.mockResolvedValue({ user: { token: null } });

    const response = await deleteHandler(makeAuthRequest(), makeContext());

    expect(response.status).toBe(401);
    const json = (await response.json()) as { code?: string };
    expect(json.code).toBe("TOKEN_EXPIRED");
  });

  it("proxies DELETE with Authorization, Content-Type, and client headers", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
      headers: new Headers(),
    });

    await deleteHandler(makeAuthRequest(), makeContext());

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/test/42",
      expect.objectContaining({
        method: "DELETE",
        headers: expect.objectContaining({
          Authorization: "Bearer access-token-123",
          "Content-Type": "application/json",
          "X-Forwarded-For": "127.0.0.1",
          "User-Agent": "test-agent",
        }) as Record<string, unknown>,
      }),
    );
  });

  it("proxies POST variant to a different dynamic endpoint", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({ ok: true }),
    });

    await postHandler(makeAuthRequest("POST"), makeContext());

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/test/42/action",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("resolves route params into the endpoint via the endpoint builder", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
      headers: new Headers(),
    });

    await deleteHandler(makeAuthRequest(), makeContext("999"));

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/test/999",
      expect.anything(),
    );
  });

  it("forwards backend JSON error with original status", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 404,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({ message: "Einladung nicht gefunden" }),
    });

    const response = await deleteHandler(makeAuthRequest(), makeContext());

    expect(response.status).toBe(404);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Einladung nicht gefunden");
  });

  it("retries with refreshed token on 401 and succeeds", async () => {
    mockAuth
      .mockResolvedValueOnce(mockSession)
      .mockResolvedValueOnce({ user: { token: "refreshed-token-456" } });

    mockFetch
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        headers: new Headers({ "content-type": "application/json" }),
        json: async () => ({ error: "Unauthorized" }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 204,
        headers: new Headers(),
      });

    const response = await deleteHandler(makeAuthRequest(), makeContext());

    expect(response.status).toBe(204);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(mockFetch).toHaveBeenLastCalledWith(
      "http://localhost:8080/test/42",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer refreshed-token-456",
        }) as Record<string, unknown>,
      }),
    );
  });

  it("returns TOKEN_EXPIRED envelope when refresh returns the same token", async () => {
    mockAuth.mockResolvedValue(mockSession);
    mockFetch.mockResolvedValue({
      ok: false,
      status: 401,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({ error: "Unauthorized" }),
    });

    const response = await deleteHandler(makeAuthRequest(), makeContext());

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string; code?: string };
    expect(json.code).toBe("TOKEN_EXPIRED");
    expect(json.error).toBe("Token expired");
  });

  it("returns empty-body 204 when backend returns 204", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
      headers: new Headers(),
    });

    const response = await deleteHandler(makeAuthRequest(), makeContext());

    expect(response.status).toBe(204);
    expect(response.body).toBeNull();
  });

  it("wraps non-JSON response in { message } envelope", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 502,
      headers: new Headers({ "content-type": "text/html" }),
      text: async () => "Bad Gateway",
    });

    const response = await deleteHandler(makeAuthRequest(), makeContext());

    expect(response.status).toBe(502);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Bad Gateway");
  });

  it("returns 500 on fetch error via handleApiError", async () => {
    mockFetch.mockRejectedValue(new Error("Network error"));

    const response = await deleteHandler(makeAuthRequest(), makeContext());

    expect(response.status).toBe(500);
  });
});
