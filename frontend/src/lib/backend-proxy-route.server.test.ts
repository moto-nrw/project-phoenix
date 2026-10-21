import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import { ApiResponseError } from "~/lib/api-response-error.server";
import { suppressConsole } from "~/test/helpers/console";
import {
  createOperatorJsonProxy,
  createOptionalTenantJsonProxy,
  createPublicJsonProxy,
  createTenantApiAdapter,
  createTenantJsonProxy,
} from "./backend-proxy-route.server";

/**
 * Owner test for the shared `create*JsonProxy` request pipeline: URL and query
 * forwarding, body modes, headers, the 401 refresh retry and error responses.
 * Route tests only pin their own configuration (path, method, options).
 */

const { mockAuth, mockUncachedAuth, mockOperatorAuth, mockUncachedOperator } =
  vi.hoisted(() => ({
    mockAuth: vi.fn(),
    mockUncachedAuth: vi.fn(),
    mockOperatorAuth: vi.fn(),
    mockUncachedOperator: vi.fn(),
  }));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockUncachedAuth,
}));
vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockOperatorAuth,
  uncachedOperatorAuth: mockUncachedOperator,
}));

const BACKEND = "http://backend.test";
const consoleSpies = suppressConsole("error");
const fetchMock = vi.fn<typeof fetch>();

const session = (token: string) => ({ user: { id: "1", token } });

function request(
  path: string,
  init: { method?: string; body?: BodyInit; headers?: HeadersInit } = {},
) {
  return new NextRequest(new URL(path, "http://localhost:3000"), init);
}

function context(params: Record<string, string> = {}) {
  return { params: Promise.resolve(params) };
}

function backendJson(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

/** URL and init of the n-th backend fetch. */
function fetchCall(index = 0) {
  const call = fetchMock.mock.calls[index];
  if (!call) throw new Error(`no fetch call #${index}`);
  return { url: String(call[0]), init: call[1] };
}

/** Headers of the n-th backend fetch, copied at call time (retries reuse the object). */
const sentHeaders: Record<string, string>[] = [];
function fetchHeaders(index = 0): Record<string, string> {
  const headers = sentHeaders[index];
  if (!headers) throw new Error(`no fetch call #${index}`);
  return headers;
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.stubEnv("API_URL", BACKEND);
  sentHeaders.length = 0;
  vi.stubGlobal("fetch", (input: RequestInfo | URL, init?: RequestInit) => {
    sentHeaders.push({ ...(init?.headers as Record<string, string>) });
    return fetchMock(input, init);
  });
  mockAuth.mockResolvedValue(session("tenant-token"));
  mockOperatorAuth.mockResolvedValue(session("operator-token"));
  fetchMock.mockImplementation(() => Promise.resolve(backendJson({ ok: 1 })));
});

afterEach(() => {
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
});

describe("createTenantJsonProxy", () => {
  it("rejects a request without a session token before calling the backend", async () => {
    mockAuth.mockResolvedValueOnce({ user: { id: "1" } });
    const GET = createTenantJsonProxy({
      method: "GET",
      path: "/api/things",
      unauthorizedError: "Nicht angemeldet",
    });

    const response = await GET(request("/api/things"), context());

    expect(response.status).toBe(401);
    expect(await response.json()).toMatchObject({
      status: 401,
      detail: "Nicht angemeldet",
    });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("sends a GET with bearer token and JSON content type and forwards the body verbatim", async () => {
    fetchMock.mockResolvedValueOnce(backendJson({ data: [1, 2] }, 200));
    const GET = createTenantJsonProxy({ method: "GET", path: "/api/things" });

    const response = await GET(request("/api/things"), context());

    expect(fetchMock).toHaveBeenCalledExactlyOnceWith(`${BACKEND}/api/things`, {
      headers: {
        Authorization: "Bearer tenant-token",
        "Content-Type": "application/json",
      },
    });
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ data: [1, 2] });
  });

  describe("query forwarding", () => {
    const url = "/api/things?page=2&tag=a&tag=b&secret=x";

    it("drops the query string by default", async () => {
      const GET = createTenantJsonProxy({ method: "GET", path: "/api/things" });
      await GET(request(url), context());
      expect(fetchCall().url).toBe(`${BACKEND}/api/things`);
    });

    it("forwards every parameter, including repeated keys, with forwardQuery: true", async () => {
      const GET = createTenantJsonProxy({
        method: "GET",
        path: "/api/things",
        forwardQuery: true,
      });
      await GET(request(url), context());
      expect(fetchCall().url).toBe(
        `${BACKEND}/api/things?page=2&tag=a&tag=b&secret=x`,
      );
    });

    it("forwards only allowlisted parameters", async () => {
      const GET = createTenantJsonProxy({
        method: "GET",
        path: "/api/things",
        forwardQuery: ["tag"],
      });
      await GET(request(url), context());
      expect(fetchCall().url).toBe(`${BACKEND}/api/things?tag=a&tag=b`);
    });
  });

  describe("path", () => {
    it("builds the backend path from the request and the awaited route params", async () => {
      const path = vi.fn(
        (_req: NextRequest, params: Record<string, unknown>) =>
          `/api/things/${String(params.id)}/children`,
      );
      const GET = createTenantJsonProxy({ method: "GET", path });

      const incoming = request("/api/things/7/children");
      await GET(incoming, context({ id: "7" }));

      expect(path).toHaveBeenCalledWith(incoming, { id: "7" });
      expect(fetchCall().url).toBe(`${BACKEND}/api/things/7/children`);
    });

    it("returns a Response from the path builder without calling the backend", async () => {
      const GET = createTenantJsonProxy({
        method: "GET",
        path: () => new Response("bad id", { status: 400 }),
      });

      const response = await GET(request("/api/things/x"), context());

      expect(response.status).toBe(400);
      expect(await response.text()).toBe("bad id");
      expect(fetchMock).not.toHaveBeenCalled();
    });
  });

  describe("body modes", () => {
    it("re-serializes a JSON body for POST", async () => {
      const POST = createTenantJsonProxy({
        method: "POST",
        path: "/api/things",
      });

      await POST(
        request("/api/things", {
          method: "POST",
          body: '{ "name" : "A" }',
        }),
        context(),
      );

      expect(fetchCall().init).toEqual({
        method: "POST",
        headers: {
          Authorization: "Bearer tenant-token",
          "Content-Type": "application/json",
        },
        body: '{"name":"A"}',
      });
    });

    it("applies mapJsonBody to the parsed body before sending it", async () => {
      const PUT = createTenantJsonProxy({
        method: "PUT",
        path: "/api/things/1",
        mapJsonBody: (body) => ({
          display_name: (body as { displayName: string }).displayName,
        }),
      });

      await PUT(
        request("/api/things/1", {
          method: "PUT",
          body: JSON.stringify({ displayName: "Raum 1" }),
        }),
        context(),
      );

      expect(fetchCall().init?.body).toBe('{"display_name":"Raum 1"}');
    });

    it("answers malformed JSON with 400 Invalid JSON and never calls the backend", async () => {
      const POST = createTenantJsonProxy({
        method: "POST",
        path: "/api/things",
      });

      const response = await POST(
        request("/api/things", { method: "POST", body: "{" }),
        context(),
      );

      expect(response.status).toBe(400);
      expect(await response.json()).toMatchObject({ detail: "Invalid JSON" });
      expect(fetchMock).not.toHaveBeenCalled();
    });

    it("uses invalidJsonMessage for malformed JSON", async () => {
      const POST = createTenantJsonProxy({
        method: "POST",
        path: "/api/things",
        invalidJsonMessage: "Ungültige Eingabe",
      });

      const response = await POST(
        request("/api/things", { method: "POST", body: "{" }),
        context(),
      );

      expect(response.status).toBe(400);
      expect(await response.json()).toMatchObject({
        detail: "Ungültige Eingabe",
      });
    });

    it("prefers invalidJsonResponse over invalidJsonMessage", async () => {
      const POST = createTenantJsonProxy({
        method: "POST",
        path: "/api/things",
        invalidJsonMessage: "unused",
        invalidJsonResponse: () =>
          Response.json({ own: true }, { status: 422 }),
      });

      const response = await POST(
        request("/api/things", { method: "POST", body: "{" }),
        context(),
      );

      expect(response.status).toBe(422);
      expect(await response.json()).toEqual({ own: true });
      expect(fetchMock).not.toHaveBeenCalled();
    });

    it("sends {} for an empty optional-json body", async () => {
      const POST = createTenantJsonProxy({
        method: "POST",
        path: "/api/things/1/archive",
        body: "optional-json",
      });

      await POST(
        request("/api/things/1/archive", { method: "POST" }),
        context(),
      );

      expect(fetchCall().init?.body).toBe("{}");
    });

    it("forwards form data without a JSON content type", async () => {
      const POST = createTenantJsonProxy({
        method: "POST",
        path: "/api/import",
        body: "form",
      });
      const form = new FormData();
      form.append("file", new Blob(["a;b"], { type: "text/csv" }), "x.csv");

      await POST(
        request("/api/import", { method: "POST", body: form }),
        context(),
      );

      const sent = fetchCall().init?.body;
      expect(sent).toBeInstanceOf(FormData);
      expect((sent as FormData).get("file")).toBeInstanceOf(File);
      expect(fetchHeaders()).toEqual({ Authorization: "Bearer tenant-token" });
    });

    it("sends no body for body: none, and an explicit undefined body with includeEmptyBody", async () => {
      const without = createTenantJsonProxy({
        method: "POST",
        path: "/api/things/1/read",
        body: "none",
      });
      const withEmpty = createTenantJsonProxy({
        method: "POST",
        path: "/api/things/1/read",
        body: "none",
        includeEmptyBody: true,
      });

      await without(request("/api/x", { method: "POST" }), context());
      await withEmpty(request("/api/x", { method: "POST" }), context());

      expect(fetchCall(0).init).not.toHaveProperty("body");
      expect(fetchCall(1).init).toHaveProperty("body", undefined);
    });
  });

  describe("method and headers", () => {
    it("omits Content-Type on GET and DELETE when disabled and sets an explicit GET method on request", async () => {
      const GET = createTenantJsonProxy({
        method: "GET",
        path: "/api/a",
        contentTypeOnGet: false,
        explicitGetMethod: true,
        cache: "no-store",
      });
      const DELETE = createTenantJsonProxy({
        method: "DELETE",
        path: "/api/a",
        contentTypeOnDelete: false,
      });

      await GET(request("/api/a"), context());
      await DELETE(request("/api/a", { method: "DELETE" }), context());

      expect(fetchCall(0).init).toEqual({
        method: "GET",
        headers: { Authorization: "Bearer tenant-token" },
        cache: "no-store",
      });
      expect(fetchCall(1).init).toEqual({
        method: "DELETE",
        headers: { Authorization: "Bearer tenant-token" },
      });
    });

    it("adds client forwarding and additional headers, but the session bearer wins", async () => {
      const POST = createTenantJsonProxy({
        method: "POST",
        path: "/api/things",
        forwardClientHeaders: true,
        additionalHeaders: (req) => ({
          Authorization: "Bearer forged",
          "X-Tenant": req.headers.get("x-tenant") ?? "",
        }),
      });

      await POST(
        request("/api/things", {
          method: "POST",
          body: "{}",
          headers: {
            "user-agent": "Browser/1",
            "x-forwarded-for": "203.0.113.9",
            "x-tenant": "schule-a",
          },
        }),
        context(),
      );

      expect(fetchHeaders()).toMatchObject({
        Authorization: "Bearer tenant-token",
        "X-Tenant": "schule-a",
        "User-Agent": "Browser/1",
        "X-Forwarded-For": "203.0.113.9",
        "Content-Type": "application/json",
      });
    });
  });

  describe("retryOn401", () => {
    it("retries once with a refreshed token", async () => {
      fetchMock
        .mockResolvedValueOnce(backendJson({ detail: "expired" }, 401))
        .mockResolvedValueOnce(backendJson({ data: "fresh" }, 200));
      mockUncachedAuth.mockResolvedValueOnce(session("new-token"));
      const GET = createTenantJsonProxy({
        method: "GET",
        path: "/api/things",
        retryOn401: true,
      });

      const response = await GET(request("/api/things"), context());

      expect(fetchMock).toHaveBeenCalledTimes(2);
      expect(fetchHeaders(0).Authorization).toBe("Bearer tenant-token");
      expect(fetchHeaders(1).Authorization).toBe("Bearer new-token");
      expect(response.status).toBe(200);
      expect(await response.json()).toEqual({ data: "fresh" });
    });

    it("forwards the 401 when the refreshed token is unchanged", async () => {
      fetchMock.mockResolvedValueOnce(backendJson({ detail: "expired" }, 401));
      mockUncachedAuth.mockResolvedValueOnce(session("tenant-token"));
      const GET = createTenantJsonProxy({
        method: "GET",
        path: "/api/things",
        retryOn401: true,
      });

      const response = await GET(request("/api/things"), context());

      expect(fetchMock).toHaveBeenCalledOnce();
      expect(response.status).toBe(401);
    });

    it("does not refresh without retryOn401", async () => {
      fetchMock.mockResolvedValueOnce(backendJson({ detail: "expired" }, 401));
      const GET = createTenantJsonProxy({ method: "GET", path: "/api/things" });

      const response = await GET(request("/api/things"), context());

      expect(mockUncachedAuth).not.toHaveBeenCalled();
      expect(fetchMock).toHaveBeenCalledOnce();
      expect(response.status).toBe(401);
    });
  });

  describe("backend responses", () => {
    it("forwards a 4xx status and body and notifies onBackendError", async () => {
      fetchMock.mockResolvedValueOnce(
        backendJson({ detail: "Name vergeben", code: "x.conflict" }, 409),
      );
      const onBackendError = vi.fn();
      const onSuccess = vi.fn();
      const POST = createTenantJsonProxy({
        method: "POST",
        path: "/api/things",
        onBackendError,
        onSuccess,
      });

      const response = await POST(
        request("/api/things", { method: "POST", body: "{}" }),
        context(),
      );

      expect(response.status).toBe(409);
      expect(await response.json()).toEqual({
        detail: "Name vergeben",
        code: "x.conflict",
      });
      expect(onBackendError).toHaveBeenCalledOnce();
      expect(onBackendError.mock.calls[0]?.[0]).toHaveProperty("status", 409);
      expect(onSuccess).not.toHaveBeenCalled();
    });

    it("keeps the backend response when onBackendError throws", async () => {
      fetchMock.mockResolvedValueOnce(backendJson({ detail: "nope" }, 403));
      const GET = createTenantJsonProxy({
        method: "GET",
        path: "/api/things",
        onBackendError: () => {
          throw new Error("observer failed");
        },
      });

      const response = await GET(request("/api/things"), context());

      expect(response.status).toBe(403);
      expect(await response.json()).toEqual({ detail: "nope" });
    });

    it("lets onSuccess shape a successful response", async () => {
      fetchMock.mockResolvedValueOnce(backendJson({ data: { id: 1 } }, 201));
      const POST = createTenantJsonProxy({
        method: "POST",
        path: "/api/things",
        onSuccess: async (res) =>
          Response.json({ wrapped: await res.json() }, { status: res.status }),
      });

      const response = await POST(
        request("/api/things", { method: "POST", body: "{}" }),
        context(),
      );

      expect(response.status).toBe(201);
      expect(await response.json()).toEqual({ wrapped: { data: { id: 1 } } });
    });

    it("answers an unreachable backend with a 500 problem", async () => {
      fetchMock.mockRejectedValueOnce(new TypeError("fetch failed"));
      const GET = createTenantJsonProxy({ method: "GET", path: "/api/things" });

      const response = await GET(request("/api/things"), context());

      expect(response.status).toBe(500);
      expect(await response.json()).toMatchObject({
        status: 500,
        detail: "Internal Server Error",
      });
      expect(consoleSpies.error).toHaveBeenCalledWith(
        "JSON backend proxy failed",
        { method: "GET", path: "/api/things", error: "fetch failed" },
      );
    });

    it("keeps the public 500 detail generic even with networkErrorMessage or networkErrorFromException", async () => {
      fetchMock
        .mockRejectedValueOnce(new TypeError("fetch failed"))
        .mockRejectedValueOnce(new TypeError("secret host down"));
      const withMessage = createTenantJsonProxy({
        method: "GET",
        path: "/api/things",
        networkErrorMessage: "Import fehlgeschlagen",
      });
      const fromException = createTenantJsonProxy({
        method: "GET",
        path: "/api/things",
        networkErrorFromException: true,
      });

      const first = await withMessage(request("/api/things"), context());
      const second = await fromException(request("/api/things"), context());

      expect(first.status).toBe(500);
      expect(await first.json()).toMatchObject({
        detail: "Internal Server Error",
      });
      expect(second.status).toBe(500);
      expect(await second.json()).toMatchObject({
        detail: "Internal Server Error",
      });
    });

    it("uses networkErrorResponse for an unreachable backend", async () => {
      fetchMock.mockRejectedValueOnce(new TypeError("fetch failed"));
      const GET = createTenantJsonProxy({
        method: "GET",
        path: "/api/things",
        networkErrorResponse: (error) =>
          Response.json({ message: (error as Error).message }, { status: 503 }),
      });

      const response = await GET(request("/api/things"), context());

      expect(response.status).toBe(503);
      expect(await response.json()).toEqual({ message: "fetch failed" });
    });
  });
});

describe("createOperatorJsonProxy", () => {
  it("rejects a request without an operator session", async () => {
    mockOperatorAuth.mockResolvedValueOnce(null);
    const GET = createOperatorJsonProxy({ method: "GET", path: "/operator/a" });

    const response = await GET(request("/api/operator/a"), context());

    expect(response.status).toBe(401);
    expect(fetchMock).not.toHaveBeenCalled();
    expect(mockAuth).not.toHaveBeenCalled();
  });

  it("uses the operator token and always retries a 401 with the refreshed operator token", async () => {
    fetchMock
      .mockResolvedValueOnce(backendJson({ detail: "expired" }, 401))
      .mockResolvedValueOnce(backendJson({ data: 1 }, 200));
    mockUncachedOperator.mockResolvedValueOnce(session("operator-new"));
    const GET = createOperatorJsonProxy({ method: "GET", path: "/operator/a" });

    const response = await GET(request("/api/operator/a"), context());

    expect(fetchHeaders(0).Authorization).toBe("Bearer operator-token");
    expect(fetchHeaders(1).Authorization).toBe("Bearer operator-new");
    expect(mockUncachedAuth).not.toHaveBeenCalled();
    expect(response.status).toBe(200);
  });
});

describe("createOptionalTenantJsonProxy", () => {
  it("calls the backend without Authorization when no session exists", async () => {
    mockAuth.mockResolvedValueOnce(null);
    const POST = createOptionalTenantJsonProxy({
      method: "POST",
      path: "/auth/register",
    });

    const response = await POST(
      request("/api/auth/register", { method: "POST", body: "{}" }),
      context(),
    );

    expect(response.status).toBe(200);
    expect(fetchHeaders()).toEqual({ "Content-Type": "application/json" });
  });

  it("attaches the session token when a session exists", async () => {
    const POST = createOptionalTenantJsonProxy({
      method: "POST",
      path: "/auth/register",
    });

    await POST(
      request("/api/auth/register", { method: "POST", body: "{}" }),
      context(),
    );

    expect(fetchHeaders().Authorization).toBe("Bearer tenant-token");
  });
});

describe("createPublicJsonProxy", () => {
  it("never reads a session or sends Authorization", async () => {
    const POST = createPublicJsonProxy({
      method: "POST",
      path: "/auth/password-reset",
    });

    await POST(
      request("/api/auth/password-reset", { method: "POST", body: "{}" }),
    );

    expect(mockAuth).not.toHaveBeenCalled();
    expect(fetchHeaders()).toEqual({ "Content-Type": "application/json" });
  });
});

describe("createTenantApiAdapter", () => {
  it("rejects a request without a session token before running the handler", async () => {
    mockAuth.mockResolvedValueOnce(null);
    const handler = vi.fn();
    const GET = createTenantApiAdapter(handler);

    const response = await GET(request("/api/a"), context());

    expect(response.status).toBe(401);
    expect(handler).not.toHaveBeenCalled();
  });

  it("passes request, token and context to the handler", async () => {
    const handler = vi.fn().mockResolvedValue(Response.json({ ok: true }));
    const GET = createTenantApiAdapter(handler);
    const incoming = request("/api/a");
    const ctx = context({ id: "3" });

    const response = await GET(incoming, ctx);

    expect(handler).toHaveBeenCalledWith(incoming, "tenant-token", ctx);
    expect(await response.json()).toEqual({ ok: true });
  });

  it("maps a backend ApiResponseError through handleApiError even with onLocalError", async () => {
    const onLocalError = vi.fn();
    const GET = createTenantApiAdapter(
      () => Promise.reject(new ApiResponseError(404, "Nicht gefunden")),
      onLocalError,
    );

    const response = await GET(request("/api/a"), context());

    expect(response.status).toBe(404);
    expect(onLocalError).not.toHaveBeenCalled();
  });

  it("uses onLocalError for a non-backend error", async () => {
    const failure = new Error("mapping failed");
    const onLocalError = vi.fn(() =>
      Response.json({ local: true }, { status: 422 }),
    );
    const GET = createTenantApiAdapter(
      () => Promise.reject(failure),
      onLocalError,
    );

    const response = await GET(request("/api/a"), context());

    expect(onLocalError).toHaveBeenCalledWith(failure, expect.any(NextRequest));
    expect(response.status).toBe(422);
  });
});
