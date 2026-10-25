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

import {
  proxyDelete,
  proxyFile,
  proxyGet,
  proxyPut,
  proxyUpload,
} from "./route-wrapper.server";

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

  // Quorum #4937: after a refresh, a business rejection is the answer, not
  // an expired session.
  it("keeps a business rejection of the retried request", async () => {
    const body =
      '{"type":"about:blank","title":"Conflict","status":409,"detail":"last","code":"traeger.last_administrator"}';
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
      .mockResolvedValueOnce(
        new Response(body, {
          status: 409,
          headers: { "Content-Type": "application/problem+json" },
        }),
      );
    mockUncachedAuth.mockResolvedValue({ user: { token: "fresh-token" } });

    const response = await proxyDelete("/traeger/accounts/7")(
      new NextRequest("http://localhost:3000/api/traeger/accounts/7", {
        method: "DELETE",
      }),
      context,
    );

    expect(response.status).toBe(409);
    expect(await response.text()).toBe(body);
  });

  it("forwards a write with its body (#4659)", async () => {
    mockFetch.mockResolvedValueOnce(
      Response.json({ status: "success", data: { id: 5 }, message: "ok" }),
    );
    const write = new NextRequest(
      "http://localhost:3000/api/traeger/holiday-periods/5",
      { method: "PUT", body: JSON.stringify({ name: "Ferien" }) },
    );

    const response = await proxyPut("/traeger/holiday-periods/5")(
      write,
      context,
    );

    expect(response.status).toBe(200);
    expect(await response.json()).toMatchObject({ data: { id: 5 } });
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/traeger/holiday-periods/5",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ name: "Ferien" }),
        headers: expect.objectContaining({ Authorization: "Bearer org-token" }),
      }),
    );
  });

  it("keeps a business refusal of the retried request instead of ending the session", async () => {
    const conflict =
      '{"type":"about:blank","title":"Conflict","status":409,"detail":"incomplete","code":"enrollment.holiday_period_incomplete"}';
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
      .mockResolvedValueOnce(
        new Response(conflict, {
          status: 409,
          headers: { "Content-Type": "application/problem+json" },
        }),
      );
    mockUncachedAuth.mockResolvedValue({ user: { token: "fresh-token" } });

    const response = await proxyGet("/traeger/holiday-periods/5")(
      request(),
      context,
    );

    expect(response.status).toBe(409);
    expect(await response.text()).toBe(conflict);
  });
});

describe("Träger file proxy (#4655)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockTraegerAuth.mockResolvedValue({ user: { token: "org-token" } });
  });

  function exportRequest() {
    return new NextRequest(
      "http://localhost:3000/api/traeger/access-log/export?from=2026-09-01&to=2026-09-09",
    );
  }

  it("streams the file with its name and the query forwarded", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response("﻿Datum;Uhrzeit\n", {
        status: 200,
        headers: {
          "Content-Type": "text/csv; charset=utf-8",
          "Content-Disposition": 'attachment; filename="zugriffsprotokoll.csv"',
        },
      }),
    );

    const response = await proxyFile("/traeger/access-log/export")(
      exportRequest(),
      context,
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("content-type")).toBe(
      "text/csv; charset=utf-8",
    );
    expect(response.headers.get("content-disposition")).toContain(
      "zugriffsprotokoll.csv",
    );
    expect(response.headers.get("cache-control")).toBe("no-store");
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/traeger/access-log/export?from=2026-09-01&to=2026-09-09",
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: "Bearer org-token" }),
      }),
    );
  });

  it("keeps the problem body of a refused export", async () => {
    mockFetch.mockResolvedValueOnce(
      Response.json(
        {
          status: 400,
          code: "traeger.access_log_export_too_large",
          detail: "x",
        },
        {
          status: 400,
          headers: { "Content-Type": "application/problem+json" },
        },
      ),
    );

    const response = await proxyFile("/traeger/access-log/export")(
      exportRequest(),
      context,
    );

    expect(response.status).toBe(400);
    expect(await response.json()).toMatchObject({
      code: "traeger.access_log_export_too_large",
    });
  });

  it("retries once with a refreshed Träger session after a backend 401", async () => {
    mockUncachedAuth.mockResolvedValue({ user: { token: "fresh-token" } });
    mockFetch
      .mockResolvedValueOnce(new Response(null, { status: 401 }))
      .mockResolvedValueOnce(
        new Response("ok", {
          status: 200,
          headers: { "Content-Type": "text/csv" },
        }),
      );

    const response = await proxyFile("/traeger/access-log/export")(
      exportRequest(),
      context,
    );

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenLastCalledWith(
      expect.any(String),
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer fresh-token",
        }),
      }),
    );
  });
});

describe("Träger upload proxy (#4653)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockTraegerAuth.mockResolvedValue({ user: { token: "org-token" } });
  });

  function upload(body: BodyInit, headers: Record<string, string>) {
    return new NextRequest(
      "http://localhost:3000/api/traeger/master-data/logo",
      {
        method: "PUT",
        body,
        headers,
      },
    );
  }

  it("forwards the multipart body once more after a refreshed session", async () => {
    const form = new FormData();
    form.append("logo", new File(["png"], "logo.png", { type: "image/png" }));
    const prepared = new Request("http://localhost", {
      method: "PUT",
      body: form,
    });
    const contentType = prepared.headers.get("content-type")!;
    const bytes = await prepared.arrayBuffer();
    mockFetch
      .mockResolvedValueOnce(new Response(null, { status: 401 }))
      .mockResolvedValueOnce(
        Response.json({ status: "success", data: { width: 1 } }),
      );
    mockUncachedAuth.mockResolvedValue({ user: { token: "fresh-token" } });

    const response = await proxyUpload("/traeger/master-data/logo", 1024)(
      upload(bytes, { "content-type": contentType }),
      context,
    );

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    const [url, init] = mockFetch.mock.calls[1]!;
    expect(url).toBe("http://localhost:8080/traeger/master-data/logo");
    expect(init).toMatchObject({
      method: "PUT",
      headers: expect.objectContaining({
        Authorization: "Bearer fresh-token",
        "Content-Type": contentType,
      }),
    });
    expect(new Uint8Array(init.body as ArrayBuffer)).toEqual(
      new Uint8Array(bytes),
    );
  });

  it("refuses an oversized or non-multipart body without calling the backend", async () => {
    const tooLarge = await proxyUpload("/traeger/master-data/logo", 10)(
      upload(new Uint8Array(70 * 1024), {
        "content-type": "multipart/form-data; boundary=x",
      }),
      context,
    );
    expect(tooLarge.status).toBe(400);
    expect(await tooLarge.json()).toMatchObject({
      code: "traeger.logo_too_large",
    });

    const notMultipart = await proxyUpload("/traeger/master-data/logo", 1024)(
      upload("{}", { "content-type": "application/json" }),
      context,
    );
    expect(notMultipart.status).toBe(400);
    expect(mockFetch).not.toHaveBeenCalled();
  });
});
