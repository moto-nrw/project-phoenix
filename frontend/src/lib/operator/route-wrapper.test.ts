import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { RouteContext } from "~/lib/route-wrapper-utils";

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

vi.mock("~/lib/client-headers", () => ({
  getClientForwardHeaders: mockGetClientForwardHeaders,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import {
  operatorApiGet,
  operatorApiPost,
  createOperatorProxyPostHandler,
} from "./route-wrapper";

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
    const { operatorApiPut } = await import("./route-wrapper");

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
    const { operatorApiDelete } = await import("./route-wrapper");

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
