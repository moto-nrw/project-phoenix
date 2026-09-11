import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockFetch } = vi.hoisted(() => ({ mockFetch: vi.fn() }));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend:8080",
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { GET } from "./route";

const context = (token: string) => ({ params: Promise.resolve({ token }) });

describe("GET /api/request-feed/[token]", () => {
  beforeEach(() => vi.clearAllMocks());

  it("liefert den RSS-Feed ohne Cache aus", async () => {
    mockFetch.mockResolvedValue(
      new Response('<rss version="2.0"></rss>', { status: 200 }),
    );

    const response = await GET(
      new NextRequest("http://schule.localhost:3000/api/request-feed/secret"),
      context("secret"),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://backend:8080/public/request-feed/secret",
      { cache: "no-store" },
    );
    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Type")).toBe(
      "application/rss+xml; charset=utf-8",
    );
    expect(response.headers.get("Cache-Control")).toBe("private, no-store");
    expect(await response.text()).toContain("<rss");
  });

  it("gibt unbekannte oder gesperrte Links einheitlich als 404 zurück", async () => {
    mockFetch.mockResolvedValue(new Response("Not Found", { status: 404 }));

    const response = await GET(
      new NextRequest("http://schule.localhost:3000/api/request-feed/revoked"),
      context("revoked"),
    );

    expect(response.status).toBe(404);
    expect(response.headers.get("Cache-Control")).toBe("private, no-store");
    expect(await response.text()).toBe("Not found");
  });

  it("kodiert den Capability-Token vor dem Weiterleiten", async () => {
    mockFetch.mockResolvedValue(new Response("Not Found", { status: 404 }));

    await GET(
      new NextRequest("http://schule.localhost:3000/api/request-feed/token"),
      context("part/with space"),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://backend:8080/public/request-feed/part%2Fwith%20space",
      { cache: "no-store" },
    );
  });
});
