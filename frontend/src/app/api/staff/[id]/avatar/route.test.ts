import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth, mockUncachedAuth, mockFetch, mockGetServerApiUrl } =
  vi.hoisted(() => ({
    mockAuth: vi.fn(),
    mockUncachedAuth: vi.fn(),
    mockFetch: vi.fn(),
    mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
  }));

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
    uncachedAuth: mockUncachedAuth,
  }),
);

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { GET } from "./route";

function imageResponse(bytes: Uint8Array<ArrayBuffer>, type: string): Response {
  return new Response(bytes, {
    status: 200,
    headers: { "content-type": type },
  });
}

/** A GET without body: bearer, no Content-Type, never cached. */
function avatarFetch(token: string) {
  return expect.objectContaining({
    method: "GET",
    cache: "no-store",
    headers: expect.objectContaining({
      Authorization: `Bearer ${token}`,
    }) as Record<string, string>,
  }) as RequestInit;
}

describe("GET /api/staff/[id]/avatar", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({ user: { token: "session-token" } });
    mockUncachedAuth.mockResolvedValue({ user: { token: "session-token" } });
  });

  it("returns 401 when unauthenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await GET(
      new NextRequest("http://localhost:3000/api/staff/42/avatar"),
      { params: Promise.resolve({ id: "42" }) },
    );

    expect(response.status).toBe(401);
    await expect(response.json()).resolves.toEqual({
      type: "about:blank",
      title: "Unauthorized",
      status: 401,
      detail: "Unauthorized",
      code: "general.permission",
    });
  });

  it("returns 400 when the staff id is missing", async () => {
    const response = await GET(
      new NextRequest("http://localhost:3000/api/staff/avatar"),
      { params: Promise.resolve({}) },
    );

    expect(response.status).toBe(400);
    await expect(response.json()).resolves.toEqual({
      type: "about:blank",
      title: "Bad Request",
      status: 400,
      detail: "invalid id",
      code: "general.input",
    });
  });

  it("returns 400 when the staff id is not numeric", async () => {
    const response = await GET(
      new NextRequest("http://localhost:3000/api/staff/not-a-number/avatar"),
      { params: Promise.resolve({ id: "not-a-number" }) },
    );

    expect(response.status).toBe(400);
    await expect(response.json()).resolves.toEqual({
      type: "about:blank",
      title: "Bad Request",
      status: 400,
      detail: "invalid id",
      code: "general.input",
    });
  });

  it("proxies avatar bytes without allowing caches to reuse stale avatars", async () => {
    const bytes = new Uint8Array([1, 2, 3, 4]);
    mockFetch.mockResolvedValue(imageResponse(bytes, "image/png"));

    const response = await GET(
      new NextRequest("http://localhost:3000/api/staff/42/avatar"),
      { params: Promise.resolve({ id: "42" }) },
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/staff/42/avatar",
      avatarFetch("session-token"),
    );
    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Type")).toBe("image/png");
    // A GET without body sends no Content-Type.
    const init = mockFetch.mock.calls[0]?.[1] as RequestInit | undefined;
    expect(init?.headers).toMatchObject({ Authorization: expect.any(String) });
    expect(init?.headers).not.toHaveProperty("Content-Type");
    expect(response.headers.get("Cache-Control")).toBe("private, max-age=300");
    expect(new Uint8Array(await response.arrayBuffer())).toEqual(bytes);
  });

  it("lets the browser keep a versioned avatar address without refetching", async () => {
    mockFetch.mockResolvedValue(
      imageResponse(new Uint8Array([1]), "image/png"),
    );

    const response = await GET(
      new NextRequest("http://localhost:3000/api/staff/42/avatar?v=42_abc.png"),
      { params: Promise.resolve({ id: "42" }) },
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/staff/42/avatar",
      expect.anything(),
    );
    expect(response.headers.get("Cache-Control")).toBe(
      "private, max-age=86400, immutable",
    );
  });

  it("passes backend error statuses through unchanged", async () => {
    mockFetch.mockResolvedValue(new Response("Not found", { status: 404 }));

    const response = await GET(
      new NextRequest("http://localhost:3000/api/staff/42/avatar"),
      { params: Promise.resolve({ id: "42" }) },
    );

    expect(response.status).toBe(404);
  });

  it("retries once with uncachedAuth when the backend returns 401", async () => {
    const bytes = new Uint8Array([9, 8, 7]);
    mockUncachedAuth.mockResolvedValueOnce({ user: { token: "fresh-token" } });
    mockFetch
      .mockResolvedValueOnce(new Response(null, { status: 401 }))
      .mockResolvedValueOnce(imageResponse(bytes, "image/webp"));

    const response = await GET(
      new NextRequest("http://localhost:3000/api/staff/42/avatar"),
      { params: Promise.resolve({ id: "42" }) },
    );

    expect(mockFetch).toHaveBeenNthCalledWith(
      1,
      "http://localhost:8080/api/staff/42/avatar",
      avatarFetch("session-token"),
    );
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      "http://localhost:8080/api/staff/42/avatar",
      avatarFetch("fresh-token"),
    );
    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Type")).toBe("image/webp");
    expect(new Uint8Array(await response.arrayBuffer())).toEqual(bytes);
  });

  it("answers 503 without the cause when the backend is unreachable", async () => {
    const thrown = new Error("network exploded");
    mockFetch.mockRejectedValue(thrown);

    const request = new NextRequest(
      "http://localhost:3000/api/staff/42/avatar",
    );
    const response = await GET(request, {
      params: Promise.resolve({ id: "42" }),
    });

    // A network failure answers 503 general.unavailable (was 500).
    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(response.status).toBe(503);
    const body = await response.text();
    expect(JSON.parse(body)).toMatchObject({
      status: 503,
      code: "general.unavailable",
    });
    expect(body).not.toContain(thrown.message);
  });
});
