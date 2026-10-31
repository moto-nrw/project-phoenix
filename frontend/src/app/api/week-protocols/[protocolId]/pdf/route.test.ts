import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth, mockUncachedAuth, mockAnalyticsHeaders, mockFetch } =
  vi.hoisted(() => ({
    mockAuth: vi.fn(),
    mockUncachedAuth: vi.fn(),
    mockAnalyticsHeaders: vi.fn(),
    mockFetch: vi.fn(),
  }));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockUncachedAuth,
}));
vi.mock("~/server/auth/tenant-route", () => ({
  withTenantAuth: <T>(handler: T) => handler,
}));
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend:8080",
}));
vi.mock("~/lib/analytics-session-header.server", () => ({
  incomingAnalyticsSessionHeaders: mockAnalyticsHeaders,
}));
vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn() }),
}));

global.fetch = mockFetch as unknown as typeof fetch;

const { GET } = await import("./route");

describe("GET /api/week-protocols/[protocolId]/pdf", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({ user: { token: "access-token" } });
    mockAnalyticsHeaders.mockResolvedValue({
      "X-POSTHOG-SESSION-ID": "4f79f286-6f14-49cb-bb84-16fd4f906eba",
    });
    mockFetch.mockResolvedValue(
      new Response("%PDF", {
        headers: { "content-type": "application/pdf" },
      }),
    );
  });

  it("forwards the active analytics session with the PDF download", async () => {
    const response = await GET(
      new NextRequest("http://localhost:3000/api/week-protocols/42/pdf"),
      { params: Promise.resolve({ protocolId: "42" }) },
    );

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://backend:8080/api/week-protocols/42/pdf",
      expect.objectContaining({
        headers: {
          Authorization: "Bearer access-token",
          "X-POSTHOG-SESSION-ID": "4f79f286-6f14-49cb-bb84-16fd4f906eba",
        },
      }),
    );
  });
});
