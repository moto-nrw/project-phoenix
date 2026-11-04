import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import { POST } from "./route";

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));

const mockFetch = vi.fn();

describe("holiday proof upload proxy (#5238)", () => {
  beforeEach(() => vi.stubGlobal("fetch", mockFetch));
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.resetAllMocks();
  });

  it("forwards only the file to the upload of the link, without a session", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(
        JSON.stringify({ status: "success", data: { token: "proof" } }),
        { status: 201, headers: { "content-type": "application/json" } },
      ),
    );
    const body = new FormData();
    body.append("file", new File(["%PDF-1.4"], "bescheid.pdf"));
    body.append("organization_id", "99");
    const response = await POST(
      new NextRequest(
        "http://parents.localhost:3000/api/holiday-registrations/a%2Fb/uploads",
        { method: "POST", body },
      ),
      { params: Promise.resolve({ token: "a/b" }) },
    );

    expect(response.status).toBe(201);
    const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toBe(
      "http://backend.test/traeger/holiday-registrations/a%2Fb/uploads",
    );
    expect(init.method).toBe("POST");
    expect(new Headers(init.headers).get("Authorization")).toBeNull();
    const sent = init.body as FormData;
    expect([...sent.keys()]).toEqual(["file"]);
    expect((sent.get("file") as File).name).toBe("bescheid.pdf");
  });
});
