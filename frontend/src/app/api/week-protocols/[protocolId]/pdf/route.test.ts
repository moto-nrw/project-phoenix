import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth, mockUncachedAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockUncachedAuth: vi.fn(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
    uncachedAuth: mockUncachedAuth,
    withTenantAuth: (handler: unknown) => handler,
  }),
);
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend:8080",
}));
vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ info: vi.fn(), error: vi.fn() }),
}));

global.fetch = mockFetch as unknown as typeof fetch;

const { GET } = await import("./route");

describe("GET /api/week-protocols/[protocolId]/pdf", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({ user: { token: "access-token" } });
    mockFetch.mockResolvedValue(
      new Response("%PDF", {
        headers: { "content-type": "application/pdf" },
      }),
    );
  });

  it("forwards the active analytics session with the PDF download", async () => {
    const response = await GET(
      new NextRequest("http://localhost:3000/api/week-protocols/42/pdf", {
        headers: {
          "x-posthog-session-id": "4f79f286-6f14-49cb-bb84-16fd4f906eba",
        },
      }),
      { params: Promise.resolve({ protocolId: "42" }) },
    );

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://backend:8080/api/week-protocols/42/pdf",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer access-token",
          "X-POSTHOG-SESSION-ID": "4f79f286-6f14-49cb-bb84-16fd4f906eba",
        }),
      }),
    );
  });
});
