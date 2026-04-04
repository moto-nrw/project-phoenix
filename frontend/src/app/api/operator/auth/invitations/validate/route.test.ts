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
    "http://localhost:3000/api/operator/auth/invitations/validate",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    },
  );
}

function createInvalidJsonRequest(): NextRequest {
  return new NextRequest(
    "http://localhost:3000/api/operator/auth/invitations/validate",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "not valid json{{{",
    },
  );
}

describe("POST /api/operator/auth/invitations/validate", () => {
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
      json: async () => ({
        status: "success",
        data: {
          email: "invited@example.com",
          display_name: "Test",
          expires_at: "2026-04-06T00:00:00Z",
        },
      }),
    });

    const request = createMockRequest({ token: "valid-token-uuid" });
    const response = await POST(request);

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/auth/invitations/validate",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          "Content-Type": "application/json",
          "X-Forwarded-For": "127.0.0.1",
        }) as Record<string, unknown>,
        body: JSON.stringify({ token: "valid-token-uuid" }),
      }),
    );
  });

  it("proxies backend error JSON with original status code", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 404,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({
        message: "Dieser Link ist abgelaufen oder ungültig",
      }),
    });

    const request = createMockRequest({ token: "expired-token" });
    const response = await POST(request);

    expect(response.status).toBe(404);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Dieser Link ist abgelaufen oder ungültig");
  });

  it("returns text body for non-JSON response", async () => {
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

  it("returns statusText when text body is empty", async () => {
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

  it("returns 500 on fetch error", async () => {
    mockFetch.mockRejectedValue(new Error("Network error"));

    const request = createMockRequest({ token: "some-token" });
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Ein interner Fehler ist aufgetreten");
  });
});
