import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

// Smoke test only — full proxy behavior (401 retry, TOKEN_EXPIRED envelope,
// 204 passthrough, non-JSON fallback, error forwarding, etc.) is covered by
// the helper tests in src/lib/operator/route-wrapper.test.ts. This file
// exists to pin the exact backend endpoint string and HTTP method that
// route.ts wires to, so a typo or method swap is caught by CI.
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

import { DELETE } from "./route";

describe("DELETE /api/operator/invitations/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({ user: { token: "access-token" } });
  });

  it("wires DELETE to /operator/invitations/{id}", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
      headers: new Headers(),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/invitations/42",
      { method: "DELETE" },
    );
    const context = { params: Promise.resolve({ id: "42" }) };
    await DELETE(request, context);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/invitations/42",
      expect.objectContaining({ method: "DELETE" }),
    );
  });
});
