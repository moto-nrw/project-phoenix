import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockSchoolAuth, mockFetch } = vi.hoisted(() => ({
  mockSchoolAuth: vi.fn(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth/school", () => ({
  schoolAuth: mockSchoolAuth,
  uncachedSchoolAuth: mockSchoolAuth,
}));
vi.mock("~/server/auth/school-route", () => ({
  withSchoolAuth: <T>(handler: T) => handler,
}));
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://localhost:8080",
}));
vi.mock("~/lib/analytics-session-header.server", () => ({
  incomingAnalyticsSessionHeaders: async () => ({}),
}));

global.fetch = mockFetch as typeof fetch;

import { proxyGet, proxyPost } from "./route-wrapper.server";

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
        '{"code":"school_conflict","details":{"id":"42"},"errors":[{"field":"name"}],"instance":"/school/items/42"}';
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
