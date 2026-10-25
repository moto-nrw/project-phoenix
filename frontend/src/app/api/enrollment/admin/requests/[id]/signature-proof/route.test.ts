import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth, mockUncachedAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockUncachedAuth: vi.fn(),
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
  incomingAnalyticsSessionHeaders: async () => ({}),
}));

global.fetch = mockFetch as unknown as typeof fetch;

const { GET } = await import("./route");

function call(id: string) {
  return GET(
    new NextRequest(
      `http://localhost:3000/api/enrollment/admin/requests/${id}/signature-proof`,
    ),
    { params: Promise.resolve({ id }) },
  );
}

// The staff proof of a signed enrollment (#4786).
describe("GET /api/enrollment/admin/requests/[id]/signature-proof", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({ user: { token: "access-token" } });
  });

  it("asks the request detail for the PDF with the staff session", async () => {
    mockFetch.mockResolvedValue(
      new Response("%PDF", { headers: { "content-type": "application/pdf" } }),
    );
    const response = await call("12");
    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://backend:8080/api/enrollment/admin/requests/12?format=pdf",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer access-token",
        }),
      }),
    );
  });

  it("refuses a malformed id and a missing session before the backend", async () => {
    expect((await call("12x")).status).toBe(400);
    mockAuth.mockResolvedValue(null);
    expect((await call("12")).status).toBe(401);
    expect(mockFetch).not.toHaveBeenCalled();
  });
});
