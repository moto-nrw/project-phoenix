import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import { GET } from "./route";
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));
const backend = vi.fn();
describe("Public holiday PDF proxy", () => {
  beforeEach(() => vi.stubGlobal("fetch", backend));
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.resetAllMocks();
  });
  it("preserves PDF bytes, inline disposition and the link's anonymous boundary", async () => {
    backend.mockResolvedValueOnce(
      new Response("%PDF-1.4", {
        headers: {
          "Content-Type": "application/pdf",
          "Content-Disposition": "inline; filename=Hinweis.pdf",
          "X-Content-Type-Options": "nosniff",
          "Cache-Control": "private, no-store",
        },
      }),
    );
    const response = await GET(
      new NextRequest(
        "http://parents.localhost/api/holiday-registrations/key/documents/81",
      ),
      { params: Promise.resolve({ token: "a/b", documentId: "81" }) },
    );
    expect(response.status).toBe(200);
    expect(await response.text()).toBe("%PDF-1.4");
    expect(response.headers.get("Content-Disposition")).toBe(
      "inline; filename=Hinweis.pdf",
    );
    expect(response.headers.get("Cache-Control")).toBe("private, no-store");
    const [url, init] = backend.mock.calls[0] as [string, RequestInit];
    expect(url).toBe(
      "http://backend.test/traeger/holiday-registrations/a%2Fb/documents/81",
    );
    // Anker: a PDF was returned through the public link, not a signed-in route.
    expect(response.headers.get("Content-Type")).toBe("application/pdf");
    expect(new Headers(init.headers).get("Authorization")).toBeNull();
  });
  it("keeps an unknown document's problem envelope and 404", async () => {
    const body = JSON.stringify({
      status: 404,
      code: "enrollment.holiday_period_not_found",
    });
    backend.mockResolvedValueOnce(
      new Response(body, {
        status: 404,
        headers: { "Content-Type": "application/problem+json" },
      }),
    );
    const response = await GET(
      new NextRequest(
        "http://parents.localhost/api/holiday-registrations/key/documents/81",
      ),
      { params: Promise.resolve({ token: "key", documentId: "81" }) },
    );
    expect(response.status).toBe(404);
    expect(await response.text()).toBe(body);
  });
});
