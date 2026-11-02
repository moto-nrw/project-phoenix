import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

import { POST } from "./route";

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));

const backend = vi.fn();
const LINK = "7.AbCdEfGhIjKlMnOpQrStUvWxYz_-0123456789abcd";

function request(link: string) {
  return new NextRequest(
    `http://localhost:3000/api/public/payroll-download/${link}`,
    { method: "POST" },
  );
}

describe("Public payroll download proxy (#4805)", () => {
  beforeEach(() => vi.stubGlobal("fetch", backend));
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.resetAllMocks();
  });

  it("forwards the POST without a session and streams the file back", async () => {
    backend.mockResolvedValueOnce(
      new Response("Personalnummer;Stunden", {
        headers: {
          "Content-Type": "text/csv; charset=utf-8",
          "Content-Disposition": "attachment; filename=zeitkonten_2026-09.csv",
          "Content-Length": "22",
          "Cache-Control": "no-store",
        },
      }),
    );

    const response = await POST(request(LINK), {
      params: Promise.resolve({ link: LINK }),
    });

    expect(response.status).toBe(200);
    expect(await response.text()).toBe("Personalnummer;Stunden");
    expect(response.headers.get("Content-Type")).toBe(
      "text/csv; charset=utf-8",
    );
    expect(response.headers.get("Content-Disposition")).toBe(
      "attachment; filename=zeitkonten_2026-09.csv",
    );
    expect(response.headers.get("Cache-Control")).toBe("private, no-store");
    const [url, init] = backend.mock.calls[0] as [string, RequestInit];
    expect(url).toBe(`http://backend.test/public/payroll-download/${LINK}`);
    expect(init.method).toBe("POST");
    expect(new Headers(init.headers).get("Authorization")).toBeNull();
  });

  it("passes the backend's 404 envelope through", async () => {
    const body = JSON.stringify({
      status: 404,
      code: "workforce.payroll_download_unavailable",
    });
    backend.mockResolvedValueOnce(
      new Response(body, {
        status: 404,
        headers: { "Content-Type": "application/problem+json" },
      }),
    );

    const response = await POST(request(LINK), {
      params: Promise.resolve({ link: LINK }),
    });

    expect(response.status).toBe(404);
    expect(await response.text()).toBe(body);
    expect(response.headers.get("Cache-Control")).toBe("private, no-store");
  });

  it.each([
    "7",
    "abc.AbCdEfGhIjKlMnOpQrStUvWx",
    "7.short",
    "7.AbCdEfGhIjKlMnOpQrStUv/../x",
    `7.${"a".repeat(129)}`,
  ])("answers a malformed link %s like a spent one, without the backend", async (link) => {
    const response = await POST(request(encodeURIComponent(link)), {
      params: Promise.resolve({ link }),
    });

    expect(response.status).toBe(404);
    const body = (await response.json()) as { code?: string };
    expect(body.code).toBe("workforce.payroll_download_unavailable");
    expect(backend).not.toHaveBeenCalled();
  });
});
