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

vi.mock("~/lib/client-headers.server", () => ({
  getClientForwardHeaders: mockGetClientForwardHeaders,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { POST } from "./route";

const FLOW_A = "11111111-1111-4111-8111-111111111111";
const FLOW_B = "22222222-2222-4222-8222-222222222222";

describe("POST /api/operator/auth/invitations/accept", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("wires POST to /operator/auth/invitations/accept", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({}),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/auth/invitations/accept",
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "x-operator-invitation-flow": FLOW_A,
        },
        body: JSON.stringify({
          display_name: "Test",
          password: "Str0ng!Pass",
          confirm_password: "Str0ng!Pass",
        }),
      },
    );
    request.cookies.set(`operator.invitation-token.${FLOW_A}`, "token-a");
    request.cookies.set(`operator.invitation-token.${FLOW_B}`, "token-b");
    const response = await POST(request);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/auth/invitations/accept",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          display_name: "Test",
          password: "Str0ng!Pass",
          confirm_password: "Str0ng!Pass",
          token: "token-a",
        }),
      }),
    );
    expect(response.headers.get("set-cookie")).toContain(
      `operator.invitation-token.${FLOW_A}=`,
    );
    expect(response.headers.get("set-cookie")).not.toContain(FLOW_B);
  });
});
