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

import { DELETE, PUT } from "./route";

function context(key: string) {
  return { params: Promise.resolve({ key }) };
}

function putRequest(key: string, body: unknown) {
  return new NextRequest(
    `http://localhost:3000/api/traeger/settings/values/${key}`,
    { method: "PUT", body: JSON.stringify(body) },
  );
}

function deleteRequest(key: string) {
  return new NextRequest(
    `http://localhost:3000/api/traeger/settings/values/${key}`,
    { method: "DELETE" },
  );
}

const KEY = "parent.news_enabled";

describe("PUT /api/traeger/settings/values/[key]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockTraegerAuth.mockResolvedValue({ user: { token: "org-token" } });
  });

  it("sends the value with the Träger token to the backend", async () => {
    mockFetch.mockResolvedValueOnce(
      Response.json({ status: "success", data: null, message: "ok" }),
    );

    const response = await PUT(putRequest(KEY, { value: false }), context(KEY));

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      `http://localhost:8080/traeger/settings/values/${KEY}`,
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ value: false }),
        headers: expect.objectContaining({ Authorization: "Bearer org-token" }),
      }),
    );
  });

  it("rejects a malformed key without calling the backend", async () => {
    const response = await PUT(
      putRequest("Bad%2FKey", { value: 1 }),
      context("Bad/Key"),
    );

    expect(response.status).toBe(400);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("answers 401 without a Träger session and never calls the backend", async () => {
    mockTraegerAuth.mockResolvedValue(null);

    const response = await PUT(putRequest(KEY, { value: true }), context(KEY));

    expect(response.status).toBe(401);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("keeps a backend refusal with its exact body", async () => {
    const body =
      '{"type":"about:blank","title":"Forbidden","status":403,"detail":"missing right","code":"general.permission"}';
    mockFetch.mockResolvedValueOnce(
      new Response(body, {
        status: 403,
        headers: { "Content-Type": "application/problem+json" },
      }),
    );

    const response = await PUT(putRequest(KEY, { value: true }), context(KEY));

    expect(response.status).toBe(403);
    expect(await response.text()).toBe(body);
  });

  it("repeats the same body with a refreshed session after a backend 401", async () => {
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
      .mockResolvedValueOnce(Response.json({ status: "success", data: null }));
    mockUncachedAuth.mockResolvedValue({ user: { token: "fresh-token" } });

    const response = await PUT(putRequest(KEY, { value: true }), context(KEY));

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenLastCalledWith(
      `http://localhost:8080/traeger/settings/values/${KEY}`,
      expect.objectContaining({
        body: JSON.stringify({ value: true }),
        headers: expect.objectContaining({
          Authorization: "Bearer fresh-token",
        }),
      }),
    );
  });
});

describe("DELETE /api/traeger/settings/values/[key]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockTraegerAuth.mockResolvedValue({ user: { token: "org-token" } });
  });

  // The shared Träger proxy answers a backend 204 with its empty envelope,
  // like the other Träger deletes (#4650).
  it("removes the Träger value and answers with the empty envelope", async () => {
    mockFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const response = await DELETE(deleteRequest(KEY), context(KEY));

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      `http://localhost:8080/traeger/settings/values/${KEY}`,
      expect.objectContaining({
        method: "DELETE",
        headers: expect.objectContaining({ Authorization: "Bearer org-token" }),
      }),
    );
  });

  it("forwards a 404 for a key the Träger cannot set", async () => {
    const body =
      '{"type":"about:blank","title":"Not Found","status":404,"detail":"unknown key","code":"settings.not_found"}';
    mockFetch.mockResolvedValueOnce(
      new Response(body, {
        status: 404,
        headers: { "Content-Type": "application/problem+json" },
      }),
    );

    const response = await DELETE(deleteRequest(KEY), context(KEY));

    expect(response.status).toBe(404);
    expect(await response.text()).toBe(body);
  });

  it("rejects a malformed key without calling the backend", async () => {
    const response = await DELETE(deleteRequest("x"), context("../schools"));

    expect(response.status).toBe(400);
    expect(mockFetch).not.toHaveBeenCalled();
  });
});
