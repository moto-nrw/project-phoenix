import { afterAll, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import type { ExtendedSession } from "~/test/route-test-helpers";

const { mockAuth, mockUncachedAuth, mockFetch, mockAnalyticsHeaders } =
  vi.hoisted(() => ({
    mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
    mockUncachedAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
    mockFetch: vi.fn<(...args: unknown[]) => Promise<Response>>(),
    mockAnalyticsHeaders: vi.fn(),
  }));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockUncachedAuth,
}));
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));
vi.mock("~/lib/analytics-session-header.server", () => ({
  incomingAnalyticsSessionHeaders: mockAnalyticsHeaders,
}));

const originalFetch = global.fetch;

beforeEach(() => {
  vi.clearAllMocks();
  global.fetch = mockFetch as unknown as typeof fetch;
  mockAnalyticsHeaders.mockResolvedValue({
    "X-POSTHOG-SESSION-ID": "4f79f286-6f14-49cb-bb84-16fd4f906eba",
  });
});

afterAll(() => {
  global.fetch = originalFetch;
});

const staleSession: ExtendedSession = {
  user: { id: "1", token: "stale-token", name: "Test User" },
  expires: "2099-01-01",
};

const refreshedSession: ExtendedSession = {
  user: { id: "1", token: "fresh-token", name: "Test User" },
  expires: "2099-01-01",
};

function uploadRequest(): NextRequest {
  const formData = new FormData();
  formData.append(
    "file",
    new File(["signed report"], "unfallanzeige.pdf", {
      type: "application/pdf",
    }),
  );
  return new NextRequest(
    new Request(
      "http://localhost:3000/api/students/42/accident-reports/17/paper-copy",
      { method: "POST", body: formData },
    ),
  );
}

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("POST /api/students/[id]/accident-reports/[documentId]/paper-copy", () => {
  it("forwards the analytics session on the upload and its refreshed retry", async () => {
    mockAuth.mockResolvedValue(staleSession);
    mockUncachedAuth.mockResolvedValue(refreshedSession);
    mockFetch
      .mockResolvedValueOnce(jsonResponse(401, { detail: "expired" }))
      .mockResolvedValueOnce(jsonResponse(200, { data: { id: "17" } }));

    const { POST } = await import("./route");
    const response = await POST(uploadRequest(), {
      params: Promise.resolve({ id: "42", documentId: "17" }),
    });

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(mockAnalyticsHeaders).toHaveBeenCalledTimes(2);
    for (const [, options] of mockFetch.mock.calls) {
      expect((options as RequestInit).headers).toMatchObject({
        "X-POSTHOG-SESSION-ID": "4f79f286-6f14-49cb-bb84-16fd4f906eba",
      });
    }
  });
});
