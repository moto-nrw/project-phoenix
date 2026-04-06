import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

// Smoke test only — full proxy behavior (401 retry, TOKEN_EXPIRED envelope,
// non-JSON fallback, error forwarding, etc.) is covered by the helper tests
// in src/lib/operator/route-wrapper.test.ts. This file exists to pin the
// exact backend endpoint string and HTTP method that route.ts wires to, so a
// typo or method swap is caught by CI.
const {
  mockFetch,
  mockGetServerApiUrl,
  mockGetClientForwardHeaders,
  mockAuth,
} = vi.hoisted(() => ({
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
  mockGetClientForwardHeaders: vi.fn(() => ({})),
  mockAuth: vi.fn(),
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: mockAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

vi.mock("~/lib/client-headers", () => ({
  getClientForwardHeaders: mockGetClientForwardHeaders,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { POST } from "./route";

describe("POST /api/operator/invitations/[id]/resend", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({ user: { token: "access-token" } });
  });

  it("wires POST to /operator/invitations/{id}/resend", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({}),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/invitations/42/resend",
      { method: "POST" },
    );
    const context = { params: Promise.resolve({ id: "42" }) };
    await POST(request, context);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/invitations/42/resend",
      expect.objectContaining({ method: "POST" }),
    );
  });
});
