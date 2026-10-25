import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import { GET, POST } from "./route";

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));

const mockFetch = vi.fn();

function backendAnswer(status: number) {
  return new Response(JSON.stringify({ status: "success", data: {} }), {
    status,
    headers: { "content-type": "application/json" },
  });
}

describe("holiday registration proxy (#4661)", () => {
  beforeEach(() => vi.stubGlobal("fetch", mockFetch));
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.resetAllMocks();
  });

  it("reads the period of the link without a session", async () => {
    mockFetch.mockResolvedValueOnce(backendAnswer(200));
    const response = await GET(
      new NextRequest(
        "http://parents.localhost:3000/api/holiday-registrations/a%2Fb",
      ),
      { params: Promise.resolve({ token: "a/b" }) },
    );
    expect(response.status).toBe(200);
    const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://backend.test/traeger/holiday-registrations/a%2Fb");
    expect(new Headers(init.headers).get("Authorization")).toBeNull();
  });

  it("forwards only the canonical client IP with the registration", async () => {
    mockFetch.mockResolvedValueOnce(backendAnswer(201));
    const response = await POST(
      new NextRequest(
        "http://parents.localhost:3000/api/holiday-registrations/key",
        {
          method: "POST",
          headers: {
            "content-type": "application/json",
            "x-forwarded-for": "203.0.113.10, 172.20.0.4",
          },
          body: JSON.stringify({ children: [] }),
        },
      ),
      { params: Promise.resolve({ token: "key" }) },
    );
    expect(response.status).toBe(201);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://backend.test/traeger/holiday-registrations/key",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          "X-Forwarded-For": "172.20.0.4",
        }) as HeadersInit,
      }),
    );
  });
});
