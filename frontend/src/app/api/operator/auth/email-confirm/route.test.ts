import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

const { mockFetch, mockGetServerApiUrl, mockGetClientForwardHeaders } =
  vi.hoisted(() => ({
    mockFetch: vi.fn(),
    mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
    mockGetClientForwardHeaders: vi.fn(() => ({
      "X-Forwarded-For": "127.0.0.1",
      "X-Real-IP": "127.0.0.1",
      "User-Agent": "test-agent",
    })),
  }));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

vi.mock("~/lib/client-headers", () => ({
  getClientForwardHeaders: mockGetClientForwardHeaders,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { POST } from "./route";

function createMockRequest(body: unknown): NextRequest {
  return new NextRequest(
    "http://localhost:3000/api/operator/auth/email-confirm",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    },
  );
}

function createInvalidJsonRequest(): NextRequest {
  return new NextRequest(
    "http://localhost:3000/api/operator/auth/email-confirm",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "not valid json{{{",
    },
  );
}

describe("POST /api/operator/auth/email-confirm", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns 400 for invalid JSON body", async () => {
    const request = createInvalidJsonRequest();
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Ungültige Anfrage");
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("proxies successful JSON response", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({ message: "E-Mail erfolgreich geändert" }),
    });

    const request = createMockRequest({
      token: "550e8400-e29b-41d4-a716-446655440000",
    });
    const response = await POST(request);

    expect(response.status).toBe(200);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("E-Mail erfolgreich geändert");
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/auth/email-confirm",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          "Content-Type": "application/json",
          "X-Forwarded-For": "127.0.0.1",
        }) as Record<string, unknown>,
        body: JSON.stringify({
          token: "550e8400-e29b-41d4-a716-446655440000",
        }),
      }),
    );
  });

  it("returns German message for 429 non-JSON response", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 429,
      headers: new Headers({ "content-type": "text/plain" }),
      text: async () => "Too Many Requests",
    });

    const request = createMockRequest({ token: "some-token" });
    const response = await POST(request);

    expect(response.status).toBe(429);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe(
      "Zu viele Anfragen. Bitte versuchen Sie es später erneut.",
    );
  });

  it("returns text body for non-JSON, non-429 response", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 502,
      headers: new Headers({ "content-type": "text/html" }),
      text: async () => "Bad Gateway",
    });

    const request = createMockRequest({ token: "some-token" });
    const response = await POST(request);

    expect(response.status).toBe(502);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Bad Gateway");
  });

  it("proxies backend error JSON with original status code", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 400,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({
        message: "Ungültiger oder abgelaufener Token",
      }),
    });

    const request = createMockRequest({ token: "expired-token" });
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Ungültiger oder abgelaufener Token");
  });

  it("returns 500 on fetch error", async () => {
    mockFetch.mockRejectedValue(new Error("Network error"));

    const request = createMockRequest({ token: "some-token" });
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Ein interner Fehler ist aufgetreten");
  });

  it("returns statusText when text body is empty for non-JSON response", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 503,
      statusText: "Service Unavailable",
      headers: new Headers({ "content-type": "text/plain" }),
      text: async () => "",
    });

    const request = createMockRequest({ token: "some-token" });
    const response = await POST(request);

    expect(response.status).toBe(503);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Service Unavailable");
  });
});
