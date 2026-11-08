import { NextRequest } from "next/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { auth, refresh } = vi.hoisted(() => ({
  auth: vi.fn(),
  refresh: vi.fn(),
}));
vi.mock("~/server/auth/caterer", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("caterer", {
    auth: auth,
    uncachedAuth: refresh,
  }),
);
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://localhost:8080",
}));

import { GET } from "./route";

beforeEach(() => {
  vi.resetAllMocks();
  auth.mockResolvedValue({ user: { token: "original" } });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

const request = () =>
  new NextRequest("http://localhost/api/caterer/orders/export?format=csv");

describe("caterer order export", () => {
  it("retries an expired token and preserves the exported bytes and filename", async () => {
    refresh.mockResolvedValue({ user: { token: "renewed" } });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response("expired", { status: 401 }))
      .mockResolvedValueOnce(
        new Response("date,count\n2026-10-21,12", {
          headers: {
            "Content-Type": "text/csv",
            "Content-Disposition": 'attachment; filename="orders.csv"',
          },
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(request(), { params: Promise.resolve({}) });

    expect(response.status).toBe(200);
    expect(await response.text()).toBe("date,count\n2026-10-21,12");
    expect(response.headers.get("Content-Type")).toBe("text/csv");
    expect(response.headers.get("Content-Disposition")).toBe(
      'attachment; filename="orders.csv"',
    );
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "http://localhost:8080/caterer/orders/export?format=csv",
      expect.objectContaining({
        method: "GET",
        cache: "no-store",
        headers: expect.objectContaining({ Authorization: "Bearer renewed" }),
      }),
    );
  });

  // No session left is a refused renewal (401); the old token kept is an
  // undecided one (503, session left as is).
  it.each([
    [null, 401],
    [{ user: { token: "original" } }, 503],
  ])(
    "does not retry without a different token (%j → %i)",
    async (session, status) => {
      refresh.mockResolvedValue(session);
      const fetchMock = vi
        .fn()
        .mockResolvedValue(new Response("expired", { status: 401 }));
      vi.stubGlobal("fetch", fetchMock);

      const response = await GET(request(), { params: Promise.resolve({}) });
      expect(response.status).toBe(status);
      expect(fetchMock).toHaveBeenCalledTimes(1);
    },
  );

  it("retries only once if the refreshed token is rejected", async () => {
    refresh.mockResolvedValue({ user: { token: "renewed" } });
    const fetchMock = vi
      .fn()
      .mockImplementation(() =>
        Promise.resolve(new Response("expired", { status: 401 })),
      );
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(request(), { params: Promise.resolve({}) });

    expect(response.status).toBe(401);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(refresh).toHaveBeenCalledTimes(1);
  });
});
