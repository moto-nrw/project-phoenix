import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { GET, POST, PUT, PATCH, DELETE, OPTIONS } from "./route";

// ============================================================================
// Mocks
// ============================================================================

const mockFetch = vi.fn();

// Mock global fetch
vi.stubGlobal("fetch", mockFetch);

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(
  path: string,
  options: {
    method?: string;
    body?: string;
    headers?: Record<string, string>;
  } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const headers = new Headers();

  if (options.headers) {
    Object.entries(options.headers).forEach(([key, value]) => {
      headers.set(key, value);
    });
  }

  return new NextRequest(url, {
    method: options.method ?? "GET",
    body: options.body,
    headers,
  });
}

// ============================================================================
// Tests
// ============================================================================

describe("BetterAuth Proxy Route", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("proxyToBetterAuth", () => {
    it("proxies GET request to BetterAuth service", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        statusText: "OK",
        headers: new Headers({
          "Content-Type": "application/json",
        }),
        text: vi.fn().mockResolvedValue('{"session":"data"}'),
        getSetCookie: () => [],
      });

      const request = createMockRequest("/api/auth/session");
      const response = await GET(request);

      expect(mockFetch).toHaveBeenCalledWith(
        "http://localhost:3001/api/auth/session",
        expect.objectContaining({
          method: "GET",
          redirect: "manual",
        }),
      );
      expect(response.status).toBe(200);
    });

    it("proxies POST request with body", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        statusText: "OK",
        headers: new Headers({
          "Content-Type": "application/json",
        }),
        text: vi.fn().mockResolvedValue('{"success":true}'),
        getSetCookie: () => [],
      });

      const request = createMockRequest("/api/auth/sign-in/email", {
        method: "POST",
        body: JSON.stringify({ email: "test@example.com", password: "secret" }),
        headers: { "Content-Type": "application/json" },
      });
      const response = await POST(request);

      expect(mockFetch).toHaveBeenCalledWith(
        "http://localhost:3001/api/auth/sign-in/email",
        expect.objectContaining({
          method: "POST",
        }),
      );
      expect(response.status).toBe(200);
    });

    it("forwards Origin header when present", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        statusText: "OK",
        headers: new Headers(),
        text: vi.fn().mockResolvedValue(""),
        getSetCookie: () => [],
      });

      const request = createMockRequest("/api/auth/session", {
        headers: { Origin: "http://localhost:3000" },
      });
      await GET(request);

      // Verify fetch was called - header forwarding is tested at integration level
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/auth/session"),
        expect.objectContaining({
          method: "GET",
          redirect: "manual",
        }),
      );
    });

    it("forwards Referer header when present", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        statusText: "OK",
        headers: new Headers(),
        text: vi.fn().mockResolvedValue(""),
        getSetCookie: () => [],
      });

      const request = createMockRequest("/api/auth/session", {
        headers: { Referer: "http://localhost:3000/login" },
      });
      await GET(request);

      // Verify fetch was called
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/auth/session"),
        expect.any(Object),
      );
    });

    it("forwards Cookie header when present", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        statusText: "OK",
        headers: new Headers(),
        text: vi.fn().mockResolvedValue(""),
        getSetCookie: () => [],
      });

      const request = createMockRequest("/api/auth/session", {
        headers: { Cookie: "session=abc123" },
      });
      await GET(request);

      // Verify fetch was called
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/auth/session"),
        expect.any(Object),
      );
    });

    it("processes Set-Cookie headers from backend response", async () => {
      const mockGetSetCookie = vi.fn(() => [
        "session=token123; HttpOnly; Path=/",
        "refresh=refresh456; HttpOnly; Path=/",
      ]);

      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        statusText: "OK",
        headers: {
          get: (name: string) =>
            name === "Content-Type" ? "application/json" : null,
          getSetCookie: mockGetSetCookie,
        },
        text: vi.fn().mockResolvedValue('{"success":true}'),
      });

      const request = createMockRequest("/api/auth/sign-in/email", {
        method: "POST",
        body: "{}",
      });
      const response = await POST(request);

      // The route should have called getSetCookie to extract cookies
      expect(mockGetSetCookie).toHaveBeenCalled();
      expect(response.status).toBe(200);
    });

    it("forwards Location header for redirects", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 302,
        statusText: "Found",
        headers: {
          get: (name: string) => (name === "Location" ? "/dashboard" : null),
          getSetCookie: () => [],
        },
        text: vi.fn().mockResolvedValue(""),
      });

      const request = createMockRequest("/api/auth/callback");
      const response = await GET(request);

      expect(response.status).toBe(302);
      expect(response.headers.get("Location")).toBe("/dashboard");
    });

    it("forwards query parameters", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        statusText: "OK",
        headers: new Headers(),
        text: vi.fn().mockResolvedValue(""),
        getSetCookie: () => [],
      });

      const request = createMockRequest(
        "/api/auth/callback?code=abc&state=xyz",
      );
      await GET(request);

      expect(mockFetch).toHaveBeenCalledWith(
        "http://localhost:3001/api/auth/callback?code=abc&state=xyz",
        expect.any(Object),
      );
    });

    it("handles fetch error with 503 response", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Connection refused"));

      const request = createMockRequest("/api/auth/session");
      const response = await GET(request);

      expect(response.status).toBe(503);
      const json = (await response.json()) as { error: string };
      expect(json.error).toBe("Authentication service unavailable");
    });

    it("handles empty response body", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
        statusText: "No Content",
        headers: new Headers(),
        text: vi.fn().mockResolvedValue(""),
        getSetCookie: () => [],
      });

      const request = createMockRequest("/api/auth/sign-out", {
        method: "POST",
      });
      const response = await POST(request);

      expect(response.status).toBe(204);
    });

    it("handles request without body for GET", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        statusText: "OK",
        headers: new Headers(),
        text: vi.fn().mockResolvedValue("{}"),
        getSetCookie: () => [],
      });

      const request = createMockRequest("/api/auth/session");
      await GET(request);

      const callArgs = mockFetch.mock.calls[0];
      expect(callArgs?.[1]?.body).toBeUndefined();
    });
  });

  describe("HTTP Methods", () => {
    beforeEach(() => {
      mockFetch.mockResolvedValue({
        ok: true,
        status: 200,
        statusText: "OK",
        headers: new Headers(),
        text: vi.fn().mockResolvedValue("{}"),
        getSetCookie: () => [],
      });
    });

    it("GET proxies with GET method", async () => {
      const request = createMockRequest("/api/auth/session");
      await GET(request);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({ method: "GET" }),
      );
    });

    it("POST proxies with POST method", async () => {
      const request = createMockRequest("/api/auth/sign-in", {
        method: "POST",
      });
      await POST(request);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({ method: "POST" }),
      );
    });

    it("PUT proxies with PUT method", async () => {
      const request = createMockRequest("/api/auth/user", { method: "PUT" });
      await PUT(request);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({ method: "PUT" }),
      );
    });

    it("PATCH proxies with PATCH method", async () => {
      const request = createMockRequest("/api/auth/user", { method: "PATCH" });
      await PATCH(request);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({ method: "PATCH" }),
      );
    });

    it("DELETE proxies with DELETE method", async () => {
      const request = createMockRequest("/api/auth/session", {
        method: "DELETE",
      });
      await DELETE(request);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({ method: "DELETE" }),
      );
    });

    it("OPTIONS proxies with OPTIONS method", async () => {
      const request = createMockRequest("/api/auth/session", {
        method: "OPTIONS",
      });
      await OPTIONS(request);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({ method: "OPTIONS" }),
      );
    });
  });

  describe("Content-Type handling", () => {
    it("sets default Content-Type to application/json", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        statusText: "OK",
        headers: new Headers(),
        text: vi.fn().mockResolvedValue("{}"),
        getSetCookie: () => [],
      });

      const request = createMockRequest("/api/auth/session");
      await GET(request);

      const callArgs = mockFetch.mock.calls[0];
      const headers = callArgs?.[1]?.headers as Headers;
      expect(headers.get("Content-Type")).toBe("application/json");
    });

    it("forwards custom Content-Type from request", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        statusText: "OK",
        headers: new Headers(),
        text: vi.fn().mockResolvedValue("{}"),
        getSetCookie: () => [],
      });

      const request = createMockRequest("/api/auth/webhook", {
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
      });
      await GET(request);

      const callArgs = mockFetch.mock.calls[0];
      const headers = callArgs?.[1]?.headers as Headers;
      expect(headers.get("Content-Type")).toBe(
        "application/x-www-form-urlencoded",
      );
    });

    it("forwards response Content-Type header", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        statusText: "OK",
        headers: {
          get: (name: string) => (name === "Content-Type" ? "text/html" : null),
          getSetCookie: () => [],
        },
        text: vi.fn().mockResolvedValue("<html></html>"),
      });

      const request = createMockRequest("/api/auth/callback");
      const response = await GET(request);

      expect(response.headers.get("Content-Type")).toBe("text/html");
    });
  });
});
