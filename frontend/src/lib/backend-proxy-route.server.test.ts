import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import { suppressConsole } from "~/test/helpers/console";
import { createPublicJsonProxy } from "./backend-proxy-route.server";
import { createOperatorJsonProxy } from "./operator/route-wrapper.server";
import {
  createOptionalTenantJsonProxy,
  createTenantJsonProxy,
} from "./route-wrapper.server";

/**
 * Owner test for the shared JSON proxy request pipeline: URL and query
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

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
    uncachedAuth: mockUncachedAuth,
  }),
);
vi.mock("~/server/auth/operator", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("operator", {
    auth: mockOperatorAuth,
    uncachedAuth: mockUncachedOperator,
  }),
);

const BACKEND = "http://backend.test";
const consoleSpies = suppressConsole("error");
const fetchMock = vi.fn<typeof fetch>();

const session = (token: string) => ({ user: { id: "1", token } });

/** The frontend origin every call for a browser request names. */
const ORIGIN = { "X-Moto-Frontend-Origin": "http://localhost:3000" };

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
    const GET = createTenantJsonProxy({ method: "GET", path: "/api/things" });

    const response = await GET(request("/api/things"), context());

    expect(response.status).toBe(401);
    expect(await response.json()).toMatchObject({
      status: 401,
      detail: "Unauthorized",
    });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("sends a GET with the bearer token, no body and no-store, and forwards the body verbatim", async () => {
    fetchMock.mockResolvedValueOnce(backendJson({ data: [1, 2] }, 200));
    const GET = createTenantJsonProxy({ method: "GET", path: "/api/things" });

    const response = await GET(request("/api/things"), context());

    expect(fetchCall().url).toBe(`${BACKEND}/api/things`);
    expect(fetchCall().init).toMatchObject({
      method: "GET",
      cache: "no-store",
      body: undefined,
    });
    expect(fetchHeaders()).toEqual({
      ...ORIGIN,
      Authorization: "Bearer tenant-token",
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

      expect(fetchCall().init).toMatchObject({
        method: "POST",
        body: '{"name":"A"}',
      });
      expect(fetchHeaders()).toEqual({
        ...ORIGIN,
        Authorization: "Bearer tenant-token",
        "Content-Type": "application/json",
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

    it("answers malformed JSON with invalidJsonResponse when given", async () => {
      const POST = createTenantJsonProxy({
        method: "POST",
        path: "/api/things",
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
      expect(fetchHeaders()).toEqual({
        ...ORIGIN,
        Authorization: "Bearer tenant-token",
      });
    });

    it("sends no body for body: none", async () => {
      const POST = createTenantJsonProxy({
        method: "POST",
        path: "/api/things/1/read",
        body: "none",
      });

      await POST(request("/api/x", { method: "POST" }), context());

      expect(fetchCall().init?.body).toBeUndefined();
      expect(fetchHeaders()).not.toHaveProperty("Content-Type");
    });
  });

  describe("method and headers", () => {
    it("names the method and sends Content-Type only with a body", async () => {
      const GET = createTenantJsonProxy({ method: "GET", path: "/api/a" });
      const DELETE = createTenantJsonProxy({
        method: "DELETE",
        path: "/api/a",
      });

      await GET(request("/api/a"), context());
      await DELETE(request("/api/a", { method: "DELETE" }), context());

      for (const [index, method] of [
        [0, "GET"],
        [1, "DELETE"],
      ] as const) {
        expect(fetchCall(index).init).toMatchObject({
          method,
          cache: "no-store",
        });
        expect(fetchHeaders(index)).toEqual({
          ...ORIGIN,
          Authorization: "Bearer tenant-token",
        });
      }
    });

    it("adds client forwarding and additional headers, but the session bearer wins", async () => {
      const POST = createTenantJsonProxy({
        method: "POST",
        path: "/api/things",
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
        ...ORIGIN,
      });
    });
  });

  describe("401 retry", () => {
    it("retries once with a refreshed token", async () => {
      fetchMock
        .mockResolvedValueOnce(backendJson({ detail: "expired" }, 401))
        .mockResolvedValueOnce(backendJson({ data: "fresh" }, 200));
      mockUncachedAuth.mockResolvedValueOnce(session("new-token"));
      const GET = createTenantJsonProxy({ method: "GET", path: "/api/things" });

      const response = await GET(request("/api/things"), context());

      expect(fetchMock).toHaveBeenCalledTimes(2);
      expect(fetchHeaders(0).Authorization).toBe("Bearer tenant-token");
      expect(fetchHeaders(1).Authorization).toBe("Bearer new-token");
      expect(response.status).toBe(200);
      expect(await response.json()).toEqual({ data: "fresh" });
    });

    it("answers 503 when the renewal kept the old token (undecided)", async () => {
      fetchMock.mockResolvedValueOnce(backendJson({ detail: "expired" }, 401));
      mockUncachedAuth.mockResolvedValueOnce(session("tenant-token"));
      const GET = createTenantJsonProxy({ method: "GET", path: "/api/things" });

      const response = await GET(request("/api/things"), context());

      expect(fetchMock).toHaveBeenCalledOnce();
      expect(response.status).toBe(503);
      expect(await response.json()).toMatchObject({
        code: "general.unavailable",
      });
    });

    it("does not refresh an endpoint that opts out, and forwards its 401", async () => {
      fetchMock.mockResolvedValueOnce(backendJson({ detail: "expired" }, 401));
      const GET = createTenantJsonProxy({
        method: "GET",
        path: "/api/things",
        retry: { off: "credential check" },
      });

      const response = await GET(request("/api/things"), context());

      expect(mockUncachedAuth).not.toHaveBeenCalled();
      expect(fetchMock).toHaveBeenCalledOnce();
      expect(response.status).toBe(401);
      expect(await response.json()).toEqual({ detail: "expired" });
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

    it("answers an unreachable backend with 503 general.unavailable", async () => {
      fetchMock.mockRejectedValueOnce(new TypeError("fetch failed"));
      const GET = createTenantJsonProxy({ method: "GET", path: "/api/things" });

      const response = await GET(request("/api/things"), context());

      expect(response.status).toBe(503);
      expect(await response.json()).toMatchObject({
        status: 503,
        code: "general.unavailable",
      });
      expect(consoleSpies.error).toHaveBeenCalledWith(
        "JSON backend proxy failed",
        { method: "GET", path: "/api/things", error: "fetch failed" },
      );
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
    expect(fetchHeaders()).toEqual({
      ...ORIGIN,
      "Content-Type": "application/json",
    });
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
    expect(fetchHeaders()).toEqual({
      ...ORIGIN,
      "Content-Type": "application/json",
    });
  });
});

describe("client header forwarding", () => {
  const clientRequest = () =>
    request("/api/tenant/resolve?slug=a", {
      headers: {
        "user-agent": "Browser/1",
        "x-forwarded-for": "198.51.100.1, 203.0.113.9",
      },
    });

  // The backend rate-limits per client IP; a route that sends none puts all
  // visitors into the frontend container's single bucket.
  it("forwards the canonical client IP and User-Agent by default", async () => {
    const GET = createPublicJsonProxy({
      method: "GET",
      path: "/auth/tenant/resolve",
    });

    await GET(clientRequest());

    expect(fetchHeaders()).toEqual({
      ...ORIGIN,
      "X-Forwarded-For": "203.0.113.9",
      "User-Agent": "Browser/1",
    });
  });
});
