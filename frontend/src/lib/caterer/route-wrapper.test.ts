import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockCatererAuth, mockFetch } = vi.hoisted(() => ({
  mockCatererAuth: vi.fn(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth/caterer", () => ({
  catererAuth: mockCatererAuth,
  uncachedCatererAuth: mockCatererAuth,
}));
vi.mock("~/server/auth/caterer-route", () => ({
  withCatererAuth: <T>(handler: T) => handler,
}));
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://localhost:8080",
}));

global.fetch = mockFetch as typeof fetch;

import { proxyPost } from "./route-wrapper.server";

describe("caterer token refresh", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("answers the retry's empty result instead of 401 after a token refresh", async () => {
    mockCatererAuth
      .mockResolvedValueOnce({ user: { token: "old-token" } })
      .mockResolvedValueOnce({ user: { token: "new-token" } });
    mockFetch
      .mockResolvedValueOnce(new Response("Unauthorized", { status: 401 }))
      .mockResolvedValueOnce(Response.json({ status: "success", data: null }));

    const response = await proxyPost("/caterer/orders/7/confirm")(
      new NextRequest("http://localhost:3000/api/caterer/orders/7/confirm", {
        method: "POST",
        body: "{}",
      }),
      { params: Promise.resolve({}) },
    );

    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(mockFetch).toHaveBeenLastCalledWith(
      "http://localhost:8080/caterer/orders/7/confirm",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer new-token",
        }),
      }),
    );
    expect(response.status).toBe(200);
  });
});
