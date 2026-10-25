import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockTraegerAuth, mockUncachedAuth, mockFetch } = vi.hoisted(() => ({
  mockTraegerAuth: vi.fn(),
  mockUncachedAuth: vi.fn(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth/traeger", () => ({
  traegerAuth: mockTraegerAuth,
  uncachedTraegerAuth: mockUncachedAuth,
}));
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://localhost:8080",
}));
vi.mock("~/lib/analytics-session-header.server", () => ({
  incomingAnalyticsSessionHeaders: async () => ({}),
}));

global.fetch = mockFetch as typeof fetch;

import { proxyDelete, proxyGet } from "./route-wrapper.server";

const context = { params: Promise.resolve({}) };

function request() {
  return new NextRequest("http://localhost:3000/api/traeger/schools");
}

describe("Träger proxy", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockTraegerAuth.mockResolvedValue({ user: { token: "org-token" } });
  });

  it("forwards the Träger token and unwraps the backend envelope", async () => {
    mockFetch.mockResolvedValueOnce(
      Response.json({ status: "success", data: [{ id: 2 }], message: "ok" }),
    );

    const response = await proxyGet("/traeger/schools")(request(), context);

    expect(response.status).toBe(200);
    expect(await response.json()).toMatchObject({ data: [{ id: 2 }] });
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/traeger/schools",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({ Authorization: "Bearer org-token" }),
      }),
    );
  });

  it("answers 401 without a Träger session and never calls the backend", async () => {
    mockTraegerAuth.mockResolvedValue(null);

    const response = await proxyGet("/traeger/schools")(request(), context);

    expect(response.status).toBe(401);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("keeps a backend refusal with its exact body", async () => {
    const body =
      '{"type":"about:blank","title":"Forbidden","status":403,"detail":"missing permission","code":"general.permission"}';
    mockFetch.mockResolvedValueOnce(
      new Response(body, {
        status: 403,
        headers: { "Content-Type": "application/problem+json" },
      }),
    );

    const response = await proxyGet("/traeger/schools")(request(), context);

    expect(response.status).toBe(403);
    expect(await response.text()).toBe(body);
  });

  it("retries once with a refreshed session after a backend 401", async () => {
    mockFetch
      .mockResolvedValueOnce(
        new Response(
          '{"status":401,"detail":"expired","code":"general.permission"}',
          {
            status: 401,
            headers: { "Content-Type": "application/problem+json" },
          },
        ),
      )
      .mockResolvedValueOnce(Response.json({ status: "success", data: [] }));
    mockUncachedAuth.mockResolvedValue({ user: { token: "fresh-token" } });

    const response = await proxyGet("/traeger/schools")(request(), context);

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenLastCalledWith(
      "http://localhost:8080/traeger/schools",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer fresh-token",
        }),
      }),
    );
  });

  // Quorum #4937: a DELETE answers null; after a refresh that is a success,
  // not an expired session.
  it("keeps a DELETE that succeeds after a refresh a success", async () => {
    mockFetch
      .mockResolvedValueOnce(
        new Response(
          '{"status":401,"detail":"expired","code":"general.permission"}',
          {
            status: 401,
            headers: { "Content-Type": "application/problem+json" },
          },
        ),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    mockUncachedAuth.mockResolvedValue({ user: { token: "fresh-token" } });

    const response = await proxyDelete("/traeger/accounts/7")(
      new NextRequest("http://localhost:3000/api/traeger/accounts/7", {
        method: "DELETE",
      }),
      context,
    );

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenLastCalledWith(
      "http://localhost:8080/traeger/accounts/7",
      expect.objectContaining({
        method: "DELETE",
        headers: expect.objectContaining({
          Authorization: "Bearer fresh-token",
        }),
      }),
    );
  });
});
