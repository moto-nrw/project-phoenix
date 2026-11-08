import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockSchoolAuth, mockFetch } = vi.hoisted(() => ({
  mockSchoolAuth: vi.fn(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth/school", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("school", {
    schoolAuth: mockSchoolAuth,
    uncachedSchoolAuth: mockSchoolAuth,
    withSchoolAuth: (handler: unknown) => handler,
  }),
);
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://localhost:8080",
}));
global.fetch = mockFetch as typeof fetch;

import { proxyDelete, proxyGet, proxyPost } from "./route-wrapper.server";

const context = { params: Promise.resolve({}) };

describe("school proxy backend errors", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSchoolAuth.mockResolvedValue({ user: { token: "school-token" } });
  });

  it.each(["GET", "POST"])(
    "forwards a %s conflict with its exact body and content type",
    async (method) => {
      const body =
        '{"type":"about:blank","title":"Conflict","status":409,"detail":"school conflict","code":"general.business_rejection","details":{"id":"42"},"errors":[{"field":"name","reason":"duplicate"}],"instance":"/school/items/42"}';
      mockFetch.mockResolvedValueOnce(
        new Response(body, {
          status: 409,
          headers: { "Content-Type": "application/problem+json" },
        }),
      );

      const request = new NextRequest(
        "http://localhost:3000/api/school/items",
        {
          method,
          ...(method === "POST"
            ? { body: JSON.stringify({ name: "conflict" }) }
            : {}),
        },
      );
      const handler =
        method === "GET"
          ? proxyGet("/school/items")
          : proxyPost("/school/items");
      const response = await handler(request, context);

      expect(response.status).toBe(409);
      expect(response.headers.get("Content-Type")).toBe(
        "application/problem+json",
      );
      expect(await response.text()).toBe(body);
      expect(mockFetch).toHaveBeenCalledWith(
        "http://localhost:8080/school/items",
        expect.objectContaining({
          method,
          headers: expect.objectContaining({
            Authorization: "Bearer school-token",
          }),
        }),
      );
    },
  );
});

describe("school token refresh", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("answers the retry's empty result with 204 instead of 401 after a token refresh", async () => {
    mockSchoolAuth
      .mockResolvedValueOnce({ user: { token: "old-token" } })
      .mockResolvedValueOnce({ user: { token: "new-token" } });
    mockFetch
      .mockResolvedValueOnce(new Response("Unauthorized", { status: 401 }))
      .mockResolvedValueOnce(Response.json({ status: "success", data: null }));

    const response = await proxyDelete("/school/push/subscriptions")(
      new NextRequest("http://localhost:3000/api/school/push/subscriptions", {
        method: "DELETE",
      }),
      context,
    );

    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(mockFetch).toHaveBeenLastCalledWith(
      "http://localhost:8080/school/push/subscriptions",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer new-token",
        }),
      }),
    );
    // A DELETE without a result answers 204 in every portal.
    expect(response.status).toBe(204);
  });
});
