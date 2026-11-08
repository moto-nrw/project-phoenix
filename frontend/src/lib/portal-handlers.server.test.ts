import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ServerSession } from "~/server/auth/session-types";
import { createMockContext } from "~/test/route-test-helpers";
import { makePortalHandlers } from "./portal-handlers.server";

vi.mock("./server-api-url", () => ({
  getServerApiUrl: () => "http://backend:8080",
}));

const fetchMock = vi.fn<typeof fetch>();
vi.stubGlobal("fetch", fetchMock);

const session = (token: string) => ({ user: { token } }) as ServerSession;

const auth = vi.fn<() => Promise<ServerSession | null>>();
const uncachedAuth = vi.fn<() => Promise<ServerSession | null>>();

const portal = makePortalHandlers({
  key: "parent",
  auth,
  uncachedAuth,
  withAuth: (handler) => (request, context) =>
    handler(request as never, context ?? createMockContext()),
});

function request(
  path: string,
  init: { method?: string; body?: unknown; headers?: HeadersInit } = {},
) {
  return new NextRequest(`http://localhost:3000${path}`, {
    method: init.method ?? "GET",
    headers: {
      "x-forwarded-for": "203.0.113.7",
      "user-agent": "Browser/1",
      ...(init.body === undefined
        ? {}
        : { "content-type": "application/json" }),
      ...init.headers,
    },
    body: init.body === undefined ? undefined : JSON.stringify(init.body),
  });
}

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });

const sent = (call = 0) => {
  const [url, init] = fetchMock.mock.calls[call]!;
  return { url, init: init!, headers: new Headers(init!.headers) };
};

beforeEach(() => {
  fetchMock.mockReset();
  auth.mockReset().mockResolvedValue(session("token-1"));
  uncachedAuth.mockReset().mockResolvedValue(session("token-2"));
});

describe("one backend call for every portal route", () => {
  it("sends the session's bearer, the client identity and no-store", async () => {
    fetchMock.mockResolvedValueOnce(json({ status: "success", data: [1] }));

    await portal.proxyGet("/parent/me/children")(
      request("/api/parent/me/children?include=guardians"),
      createMockContext(),
    );

    const { url, init, headers } = sent();
    expect(url).toBe(
      "http://backend:8080/parent/me/children?include=guardians",
    );
    expect(init.cache).toBe("no-store");
    expect(headers.get("authorization")).toBe("Bearer token-1");
    expect(headers.get("x-forwarded-for")).toBe("203.0.113.7");
    expect(headers.get("user-agent")).toBe("Browser/1");
    expect(headers.get("x-moto-frontend-origin")).toBe("http://localhost:3000");
  });

  it("forwards the browser's Idempotency-Key on a POST", async () => {
    fetchMock.mockResolvedValueOnce(json({ status: "success", data: {} }));

    await portal.proxyPost("/parent/me/sick-notes")(
      request("/api/parent/me/sick-notes", {
        method: "POST",
        body: { day: "2026-09-09" },
        headers: { "Idempotency-Key": "key-1" },
      }),
      createMockContext(),
    );

    expect(sent().headers.get("idempotency-key")).toBe("key-1");
    expect(sent().init.body).toBe(JSON.stringify({ day: "2026-09-09" }));
  });

  it("refuses a dot segment from a route param before any request", async () => {
    const response = await portal.proxyGet(
      (params) => `/parent/me/children/${String(params.id)}`,
    )(request("/api/parent/me/children/x"), createMockContext({ id: ".." }));

    expect(response.status).toBe(400);
    // Anker: the refusal is the BFF's own problem body.
    await expect(response.json()).resolves.toMatchObject({ status: 400 });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("answers 503 when the backend sends no headers in time", async () => {
    vi.useFakeTimers();
    fetchMock.mockImplementationOnce(
      (_url, init) =>
        new Promise((_resolve, reject) => {
          init!.signal!.addEventListener("abort", () =>
            reject(init!.signal!.reason as Error),
          );
        }),
    );

    const pending = portal.proxyGet("/parent/me/children")(
      request("/api/parent/me/children"),
      createMockContext(),
    );
    await vi.advanceTimersByTimeAsync(90_000);

    const response = await pending;
    expect(response.status).toBe(503);
    await expect(response.json()).resolves.toMatchObject({
      code: "general.unavailable",
    });
  });
});

describe("one response contract", () => {
  it("answers the envelope's data, or the whole body with raw", async () => {
    fetchMock
      .mockResolvedValueOnce(json({ status: "success", data: [1] }))
      .mockResolvedValueOnce(
        json({ status: "success", data: [1], pagination: { page: 1 } }),
      );

    const unwrapped = await portal.proxyGet("/x")(
      request("/api/x"),
      createMockContext(),
    );
    const raw = await portal.proxyGet("/x", { raw: true })(
      request("/api/x"),
      createMockContext(),
    );

    await expect(unwrapped.json()).resolves.toEqual({
      success: true,
      message: "Success",
      data: [1],
    });
    await expect(raw.json()).resolves.toMatchObject({
      data: { pagination: { page: 1 } },
    });
  });

  it("answers a DELETE without a result with 204", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const response = await portal.proxyDelete("/parent/me/push")(
      request("/api/parent/me/push", { method: "DELETE" }),
      createMockContext(),
    );

    expect(response.status).toBe(204);
    expect(sent().init.method).toBe("DELETE");
  });

  it("passes a backend error through with its status and code", async () => {
    fetchMock.mockResolvedValueOnce(
      json(
        {
          type: "about:blank",
          title: "Forbidden",
          status: 403,
          detail: "no",
          code: "general.permission",
        },
        403,
      ),
    );

    const response = await portal.proxyGet("/x")(
      request("/api/x"),
      createMockContext(),
    );

    expect(response.status).toBe(403);
    await expect(response.json()).resolves.toMatchObject({
      code: "general.permission",
    });
  });
});

describe("401 retry", () => {
  const problem401 = () =>
    json({ status: 401, title: "Unauthorized", code: "identity.x" }, 401);

  it("repeats a 401 once with the renewed session's token", async () => {
    fetchMock
      .mockResolvedValueOnce(problem401())
      .mockResolvedValueOnce(json({ status: "success", data: "ok" }));

    const response = await portal.proxyGet("/x")(
      request("/api/x"),
      createMockContext(),
    );

    expect(response.status).toBe(200);
    expect(sent(1).headers.get("authorization")).toBe("Bearer token-2");
  });

  // #5194/#5399: a refused renewal ends the session, the browser signs out.
  it.each([
    [
      "an ended session (RefreshTokenError)",
      () =>
        uncachedAuth.mockResolvedValue({
          user: { token: "" },
          error: "RefreshTokenError",
        } as ServerSession),
    ],
    ["no session left", () => uncachedAuth.mockResolvedValue(null)],
  ])("answers 401 Token expired after %s", async (_case, arrange) => {
    fetchMock.mockResolvedValueOnce(problem401());
    arrange();

    const viaHandler = await portal.proxyGet("/x")(
      request("/api/x"),
      createMockContext(),
    );
    fetchMock.mockResolvedValueOnce(problem401());
    const viaProxy = await portal.jsonProxy({ method: "GET", path: "/x" })(
      request("/api/x"),
      createMockContext(),
    );

    for (const response of [viaHandler, viaProxy]) {
      expect(response.status).toBe(401);
      await expect(response.json()).resolves.toMatchObject({
        detail: "Token expired",
      });
    }
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  // The refresh could not be decided (backend down, timeout, 5xx): the
  // session stays as it is and its own refresh decides later.
  it.each([
    [
      "a refresh that kept the old token",
      () => uncachedAuth.mockResolvedValue(session("token-1")),
    ],
    [
      "a renewal that threw",
      () => uncachedAuth.mockRejectedValue(new Error("session store down")),
    ],
  ])("answers 503 general.unavailable after %s", async (_case, arrange) => {
    fetchMock.mockResolvedValueOnce(problem401());
    arrange();

    const viaHandler = await portal.proxyGet("/x")(
      request("/api/x"),
      createMockContext(),
    );
    fetchMock.mockResolvedValueOnce(problem401());
    const viaProxy = await portal.jsonProxy({ method: "GET", path: "/x" })(
      request("/api/x"),
      createMockContext(),
    );

    for (const response of [viaHandler, viaProxy]) {
      expect(response.status).toBe(503);
      await expect(response.json()).resolves.toMatchObject({
        code: "general.unavailable",
      });
    }
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  // A refusal the backend repeats for the renewed token is its decision, not
  // an ended session: its code reaches the browser (identity.mfa_session_required).
  it("keeps the backend's body when the renewed token is refused again", async () => {
    const refusal = () =>
      json(
        {
          type: "about:blank",
          title: "Unauthorized",
          status: 401,
          detail: "second factor needed",
          code: "identity.mfa_session_required",
        },
        401,
      );
    fetchMock
      .mockResolvedValueOnce(problem401())
      .mockResolvedValueOnce(refusal())
      .mockResolvedValueOnce(problem401())
      .mockResolvedValueOnce(refusal());

    const viaHandler = await portal.proxyGet("/x")(
      request("/api/x"),
      createMockContext(),
    );
    uncachedAuth.mockResolvedValue(session("token-3"));
    const viaProxy = await portal.jsonProxy({ method: "GET", path: "/x" })(
      request("/api/x"),
      createMockContext(),
    );

    for (const response of [viaHandler, viaProxy]) {
      expect(response.status).toBe(401);
      await expect(response.json()).resolves.toMatchObject({
        code: "identity.mfa_session_required",
      });
    }
  });

  it("answers the retry's own result: a DELETE's 204 or a business refusal", async () => {
    fetchMock
      .mockResolvedValueOnce(problem401())
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(problem401())
      .mockResolvedValueOnce(
        json(
          { status: 403, title: "Forbidden", code: "general.permission" },
          403,
        ),
      );

    const deleted = await portal.proxyDelete("/x")(
      request("/api/x", { method: "DELETE" }),
      createMockContext(),
    );
    uncachedAuth.mockResolvedValue(session("token-3"));
    const refused = await portal.proxyGet("/x")(
      request("/api/x"),
      createMockContext(),
    );

    expect(deleted.status).toBe(204);
    expect(refused.status).toBe(403);
    await expect(refused.json()).resolves.toMatchObject({
      code: "general.permission",
    });
  });

  it("does not run a handler again that opts out, and forwards its 401", async () => {
    fetchMock.mockResolvedValueOnce(problem401());
    const handler = vi.fn(async (_request, body: unknown, token: string) =>
      portal.api("POST", "/operator/profile/password", token, body),
    );

    const response = await portal.post(handler, {
      retry: { off: "credential" },
    })(
      request("/api/operator/profile/password", {
        method: "POST",
        body: { current_password: "x" },
      }),
      createMockContext(),
    );

    expect(handler).toHaveBeenCalledOnce();
    expect(uncachedAuth).not.toHaveBeenCalled();
    expect(response.status).toBe(401);
  });

  it("does not repeat an endpoint that opts out, and forwards its 401", async () => {
    fetchMock.mockResolvedValueOnce(problem401());

    const response = await portal.jsonProxy({
      method: "POST",
      path: "/caterer/account/password",
      retry: { off: "credential" },
    })(
      request("/api/caterer/account/password", {
        method: "POST",
        body: { current_password: "x" },
      }),
      createMockContext(),
    );

    expect(fetchMock).toHaveBeenCalledOnce();
    expect(uncachedAuth).not.toHaveBeenCalled();
    await expect(response.json()).resolves.toMatchObject({
      code: "identity.x",
    });
  });

  it("answers 401 Unauthorized without a session and calls nothing", async () => {
    auth.mockResolvedValue(null);

    const response = await portal.jsonProxy({ method: "GET", path: "/x" })(
      request("/api/x"),
      createMockContext(),
    );

    expect(response.status).toBe(401);
    await expect(response.json()).resolves.toMatchObject({
      detail: "Unauthorized",
    });
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("file responses", () => {
  it("streams the file with its headers, nosniff and no-store", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response("a;b", {
        headers: {
          "Content-Type": "text/csv",
          "Content-Disposition": 'attachment; filename="x.csv"',
          "Content-Length": "3",
          "Set-Cookie": "backend=1",
        },
      }),
    );

    const response = await portal.proxyFile("/export", {
      forwardQuery: ["month"],
    })(request("/api/export?month=2026-09&other=1"), createMockContext());

    expect(sent().url).toBe("http://backend:8080/export?month=2026-09");
    expect(response.headers.get("content-type")).toBe("text/csv");
    expect(response.headers.get("content-disposition")).toBe(
      'attachment; filename="x.csv"',
    );
    expect(response.headers.get("content-length")).toBe("3");
    expect(response.headers.get("x-content-type-options")).toBe("nosniff");
    expect(response.headers.get("cache-control")).toBe("no-store");
    expect(response.headers.get("set-cookie")).toBeNull();
    await expect(response.text()).resolves.toBe("a;b");
  });

  it("sends the client identity and a POST body again on the retry", async () => {
    fetchMock
      .mockResolvedValueOnce(json({ status: 401 }, 401))
      .mockResolvedValueOnce(new Response("csv"));

    await portal.proxyFile("/students/export", { method: "POST" })(
      request("/api/students/export", {
        method: "POST",
        body: { format: "csv" },
      }),
      createMockContext(),
    );

    for (const call of [0, 1]) {
      expect(sent(call).init.method).toBe("POST");
      expect(sent(call).init.body).toBe('{"format":"csv"}');
      expect(sent(call).headers.get("content-type")).toBe("application/json");
      expect(sent(call).headers.get("x-forwarded-for")).toBe("203.0.113.7");
    }
  });

  it("keeps a backend error's content type and Retry-After", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({ status: 429, code: "general.rate_limit" }),
        {
          status: 429,
          headers: {
            "Content-Type": "application/problem+json",
            "Retry-After": "30",
          },
        },
      ),
    );

    const response = await portal.proxyFile("/export")(
      request("/api/export"),
      createMockContext(),
    );

    expect(response.status).toBe(429);
    expect(response.headers.get("content-type")).toBe(
      "application/problem+json",
    );
    expect(response.headers.get("retry-after")).toBe("30");
  });

  it.each([
    ["a success without a body", () => new Response(null), 502],
    [
      "a network failure",
      () => {
        throw new TypeError("fetch failed");
      },
      503,
    ],
  ])("answers %s with %s", async (_case, backend, status) => {
    fetchMock.mockImplementationOnce(async () => backend());

    const response = await portal.proxyFile("/export")(
      request("/api/export"),
      createMockContext(),
    );

    expect(response.status).toBe(status);
  });

  it("forwards the backend's sandbox CSP only for an uploaded file", async () => {
    const withCsp = () =>
      new Response("pdf", {
        headers: { "Content-Security-Policy": "sandbox; default-src 'none'" },
      });
    fetchMock.mockResolvedValueOnce(withCsp()).mockResolvedValueOnce(withCsp());

    const upload = await portal.proxyFile("/upload", { uploadedFile: true })(
      request("/api/upload"),
      createMockContext(),
    );
    const export_ = await portal.proxyFile("/export")(
      request("/api/export"),
      createMockContext(),
    );

    expect(upload.headers.get("content-security-policy")).toBe(
      "sandbox; default-src 'none'",
    );
    // Anker: the export is a real file answer.
    expect(await export_.text()).toBe("pdf");
    expect(export_.headers.get("content-security-policy")).toBeNull();
  });

  it("answers a backend error as its problem body", async () => {
    fetchMock.mockResolvedValueOnce(
      json({ status: 404, title: "Not Found", code: "general.not_found" }, 404),
    );

    const response = await portal.proxyFile("/export")(
      request("/api/export"),
      createMockContext(),
    );

    expect(response.status).toBe(404);
    await expect(response.json()).resolves.toMatchObject({
      code: "general.not_found",
    });
  });

  it("repeats a file request after a 401 with the renewed token", async () => {
    fetchMock
      .mockResolvedValueOnce(json({ status: 401 }, 401))
      .mockResolvedValueOnce(new Response("pdf"));

    const response = await portal.proxyFile("/proof")(
      request("/api/proof"),
      createMockContext(),
    );

    expect(sent(1).headers.get("authorization")).toBe("Bearer token-2");
    await expect(response.text()).resolves.toBe("pdf");
  });

  it("refuses a POST body above the limit with 413", async () => {
    const response = await portal.proxyFile("/letters", {
      method: "POST",
      maxBodyBytes: 8,
    })(
      request("/api/letters", { method: "POST", body: { text: "too long" } }),
      createMockContext(),
    );

    expect(response.status).toBe(413);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
