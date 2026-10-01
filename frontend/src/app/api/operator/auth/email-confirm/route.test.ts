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

vi.mock("~/lib/client-headers.server", () => ({
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
    mockFetch.mockResolvedValue(
      Response.json({ message: "E-Mail erfolgreich geändert" }),
    );

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
    mockFetch.mockResolvedValue(
      new Response("Too Many Requests", {
        status: 429,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest({ token: "some-token" });
    const response = await POST(request);

    expect(response.status).toBe(429);
    expect(await response.text()).toBe("Too Many Requests");
  });

  it("returns text body for non-JSON, non-429 response", async () => {
    mockFetch.mockResolvedValue(
      new Response("Bad Gateway", {
        status: 502,
        headers: { "Content-Type": "text/html" },
      }),
    );

    const request = createMockRequest({ token: "some-token" });
    const response = await POST(request);

    expect(response.status).toBe(502);
    expect(await response.text()).toBe("Bad Gateway");
  });

  it("proxies backend error JSON with original status code", async () => {
    mockFetch.mockResolvedValue(
      Response.json(
        { message: "Ungültiger oder abgelaufener Token" },
        { status: 400 },
      ),
    );

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
    mockFetch.mockResolvedValue(
      new Response("", {
        status: 503,
        statusText: "Service Unavailable",
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest({ token: "some-token" });
    const response = await POST(request);

    expect(response.status).toBe(503);
    expect(await response.text()).toBe("");
  });
});
