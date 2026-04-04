import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

// Smoke test only — full proxy behavior (body parsing, 429 handling, non-JSON
// fallback, error forwarding, etc.) is covered by the helper tests in
// src/lib/operator/route-wrapper.test.ts. This file exists to pin the exact
// backend endpoint string and HTTP method that route.ts wires to, so a typo
// or method swap is caught by CI.
const { mockFetch, mockGetServerApiUrl, mockGetClientForwardHeaders } =
  vi.hoisted(() => ({
    mockFetch: vi.fn(),
    mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
    mockGetClientForwardHeaders: vi.fn(() => ({})),
  }));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

vi.mock("~/lib/client-headers", () => ({
  getClientForwardHeaders: mockGetClientForwardHeaders,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { POST } from "./route";

describe("POST /api/operator/auth/invitations/validate", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("wires POST to /operator/auth/invitations/validate", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({}),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/auth/invitations/validate",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token: "abc" }),
      },
    );
    await POST(request);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/auth/invitations/validate",
      expect.objectContaining({ method: "POST" }),
    );
  });
});
