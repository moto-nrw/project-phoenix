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
  withTenantAuth: (handler: unknown) => handler,
}));
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend:8080",
}));

global.fetch = mockFetch as unknown as typeof fetch;

const { GET } = await import("./route");

function call() {
  return GET(
    new NextRequest("http://localhost:3000/api/traeger-documents/7/file", {
      headers: {
        "x-forwarded-for": "203.0.113.9",
        "user-agent": "Elternbrief-Test",
      },
    }),
    { params: Promise.resolve({ id: "7" }) },
  );
}

const clientIdentity = expect.objectContaining({
  "X-Forwarded-For": "203.0.113.9",
  "User-Agent": "Elternbrief-Test",
});

// The school's download of a Träger document (#4692) is rate-limited and
// audited per visitor, so both attempts carry the client's identity.
describe("GET /api/traeger-documents/[id]/file", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({ user: { token: "old-token" } });
    mockUncachedAuth.mockResolvedValue({ user: { token: "new-token" } });
  });

  it("forwards the client identity on the first try and after a refresh", async () => {
    mockFetch
      .mockResolvedValueOnce(new Response(null, { status: 401 }))
      .mockResolvedValueOnce(
        new Response("%PDF", {
          headers: { "content-type": "application/pdf" },
        }),
      );
    const response = await call();
    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(mockFetch).toHaveBeenNthCalledWith(
      1,
      "http://backend:8080/api/traeger-documents/7/file",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer old-token",
        }),
      }),
    );
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      "http://backend:8080/api/traeger-documents/7/file",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer new-token",
        }),
      }),
    );
    for (const [, init] of mockFetch.mock.calls) {
      expect((init as RequestInit).headers).toEqual(clientIdentity);
    }
  });

  it("answers an unreachable backend with the error envelope and no-store", async () => {
    mockFetch.mockRejectedValue(new TypeError("fetch failed"));
    const response = await call();
    expect(response.status).toBe(500);
    expect(response.headers.get("cache-control")).toContain("no-store");
    const body = (await response.json()) as { status?: number; code?: string };
    expect(body.status).toBe(500);
    expect(body.code).toBeTruthy();
  });
});
