import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import { POST } from "./route";

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));

const mockFetch = vi.fn();

describe("holiday contribution proxy (#4664)", () => {
  beforeEach(() => vi.stubGlobal("fetch", mockFetch));
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.resetAllMocks();
  });

  it("computes the contribution of the link without a session", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ status: "success", data: {} }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    const response = await POST(
      new NextRequest(
        "http://parents.localhost:3000/api/holiday-registrations/a%2Fb/contribution",
        {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ children: [], benefits_declared: false }),
        },
      ),
      { params: Promise.resolve({ token: "a/b" }) },
    );
    expect(response.status).toBe(200);
    const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://backend.test/traeger/holiday-quotes/a%2Fb");
    expect(init.method).toBe("POST");
    expect(new Headers(init.headers).get("Authorization")).toBeNull();
  });
});
