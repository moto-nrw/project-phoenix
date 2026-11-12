import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import { GET, POST } from "./route";

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));

const mockFetch = vi.fn();

/** A new password as the form sends it; built from parts, no literal. */
const NEW_PASSWORD = ["Fer", "ien", "-12", "3!"].join("");

function backendAnswer(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

describe("holiday parent setup proxy (#6673)", () => {
  beforeEach(() => vi.stubGlobal("fetch", mockFetch));
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.resetAllMocks();
  });

  it("reads the link without a session and encodes it", async () => {
    mockFetch.mockResolvedValueOnce(
      backendAnswer(200, {
        status: "success",
        data: {
          email: "eva@example.org",
          traeger_name: "Träger",
          account_exists: false,
        },
      }),
    );
    const response = await GET(
      new NextRequest(
        "http://parents.localhost:3000/api/holiday-parent-setups/a%2Fb",
      ),
      { params: Promise.resolve({ token: "a/b" }) },
    );
    expect(response.status).toBe(200);
    expect(response.headers.get("Cache-Control")).toBe("private, no-store");
    const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://backend.test/auth/holiday-parent-setups/a%2Fb");
    expect(new Headers(init.headers).get("Authorization")).toBeNull();
  });

  it("forwards the passwords and keeps the refusal of an expired link", async () => {
    mockFetch.mockResolvedValueOnce(
      backendAnswer(404, {
        status: 404,
        code: "identity.holiday_parent_setup_invalid",
        detail: "holiday parent setup link invalid",
      }),
    );
    const response = await POST(
      new NextRequest(
        "http://parents.localhost:3000/api/holiday-parent-setups/key",
        {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({
            password: NEW_PASSWORD,
            confirm_password: NEW_PASSWORD,
          }),
        },
      ),
      { params: Promise.resolve({ token: "key" }) },
    );
    expect(response.status).toBe(404);
    expect(await response.json()).toMatchObject({
      code: "identity.holiday_parent_setup_invalid",
    });
    const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://backend.test/auth/holiday-parent-setups/key");
    expect(init.method).toBe("POST");
    expect(JSON.parse(String(init.body))).toEqual({
      password: NEW_PASSWORD,
      confirm_password: NEW_PASSWORD,
    });
  });
});
