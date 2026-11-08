import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  type ExtendedSession,
  createMockContext,
  defaultSession,
} from "~/test/route-test-helpers";
import {
  proxyDelete,
  proxyGet,
  proxyPatch,
  proxyPost,
  proxyPut,
} from "./route-proxy.server";

const { mockAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockAuth,
  withTenantAuth: (handler: unknown) => handler,
}));
vi.mock("./server-api-url", () => ({
  getServerApiUrl: () => "http://backend:8080",
}));

const fetchMock = vi.fn<typeof fetch>();
vi.stubGlobal("fetch", fetchMock);

function request(
  path: string,
  options: { method?: string; body?: unknown; headers?: HeadersInit } = {},
): NextRequest {
  return new NextRequest(new URL(path, "http://localhost:3000"), {
    method: options.method ?? "GET",
    headers: {
      ...(options.body === undefined
        ? {}
        : { "Content-Type": "application/json" }),
      ...options.headers,
    },
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });
}

const backend = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });

const sent = (index = 0) => {
  const [url, init] = fetchMock.mock.calls[index]!;
  return { url, init: init!, headers: new Headers(init!.headers) };
};

beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockImplementation(async () =>
    backend({ status: "success", data: [{ id: 1 }] }),
  );
  mockAuth.mockReset().mockResolvedValue(defaultSession);
});

describe("proxyGet", () => {
  it("unwraps the backend data envelope by default", async () => {
    const response = await proxyGet("/api/items")(
      request("/api/items"),
      createMockContext(),
    );

    expect(sent().url).toBe("http://backend:8080/api/items");
    expect(sent().headers.get("authorization")).toBe("Bearer test-token");
    expect(await response.json()).toEqual({
      success: true,
      message: "Success",
      data: [{ id: 1 }],
    });
  });

  it("forwards the query string to the backend", async () => {
    await proxyGet("/api/items")(
      request("/api/items?page=2&limit=50"),
      createMockContext(),
    );

    expect(sent().url).toBe("http://backend:8080/api/items?page=2&limit=50");
  });

  it("forwards only the allowlisted query parameters", async () => {
    await proxyGet("/api/items", { forwardQuery: ["page"] })(
      request("/api/items?page=2&secret=x"),
      createMockContext(),
    );

    expect(sent().url).toBe("http://backend:8080/api/items?page=2");
  });

  it("sends no query string when no allowlisted parameter is set", async () => {
    await proxyGet("/api/items", { forwardQuery: ["page"] })(
      request("/api/items?secret=x"),
      createMockContext(),
    );

    expect(sent().url).toBe("http://backend:8080/api/items");
  });

  it("forwards the whole response when raw is set", async () => {
    fetchMock.mockResolvedValueOnce(
      backend({ status: "success", data: [], pagination: { page: 1 } }),
    );

    const response = await proxyGet("/api/items", { raw: true })(
      request("/api/items"),
      createMockContext(),
    );

    expect(await response.json()).toMatchObject({
      data: { data: [], pagination: { page: 1 } },
    });
  });

  it("resolves an endpoint builder against route params (id in path)", async () => {
    await proxyGet((params) => `/api/items/${String(params.id)}`)(
      request("/api/items/7"),
      createMockContext({ id: "7" }),
    );

    expect(sent().url).toBe("http://backend:8080/api/items/7");
  });

  it("returns 401 when unauthenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await proxyGet("/api/items")(
      request("/api/items"),
      createMockContext(),
    );

    expect(response.status).toBe(401);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("proxyPost", () => {
  it("forwards the body and unwraps data by default", async () => {
    fetchMock.mockResolvedValueOnce(
      backend({ status: "success", data: { id: 3 } }),
    );

    const response = await proxyPost("/api/items")(
      request("/api/items", { method: "POST", body: { name: "A" } }),
      createMockContext(),
    );

    expect(sent().init.method).toBe("POST");
    expect(sent().init.body).toBe('{"name":"A"}');
    expect(await response.json()).toMatchObject({ data: { id: 3 } });
  });

  it("rejects a body over maxBodyBytes before calling the backend", async () => {
    const response = await proxyPost("/api/items", { maxBodyBytes: 8 })(
      request("/api/items", { method: "POST", body: { name: "too long" } }),
      createMockContext(),
    );

    expect(response.status).toBe(413);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("forwards a body within maxBodyBytes", async () => {
    await proxyPost("/api/items", { maxBodyBytes: 1024 })(
      request("/api/items", { method: "POST", body: { name: "A" } }),
      createMockContext(),
    );

    expect(sent().init.body).toBe('{"name":"A"}');
  });

  it("forwards an idempotency key to the backend request", async () => {
    await proxyPost("/api/items")(
      request("/api/items", {
        method: "POST",
        body: { name: "A" },
        headers: { "Idempotency-Key": "abc" },
      }),
      createMockContext(),
    );

    expect(sent().headers.get("idempotency-key")).toBe("abc");
  });

  it("forwards the whole response when raw is set", async () => {
    fetchMock.mockResolvedValueOnce(
      backend({ status: "success", data: { id: 3 }, message: "created" }),
    );

    const response = await proxyPost("/api/items", { raw: true })(
      request("/api/items", { method: "POST", body: {} }),
      createMockContext(),
    );

    expect(await response.json()).toMatchObject({
      data: { message: "created" },
    });
  });
});

describe("proxyPut", () => {
  it("resolves the endpoint builder, forwards the body, and can return raw", async () => {
    fetchMock.mockResolvedValueOnce(
      backend({ status: "success", data: { id: 7 } }),
    );

    const response = await proxyPut(
      (params) => `/api/items/${String(params.id)}`,
      { raw: true },
    )(
      request("/api/items/7", { method: "PUT", body: { name: "B" } }),
      createMockContext({ id: "7" }),
    );

    expect(sent().url).toBe("http://backend:8080/api/items/7");
    expect(sent().init).toMatchObject({ method: "PUT", body: '{"name":"B"}' });
    expect(await response.json()).toMatchObject({
      data: { status: "success" },
    });
  });
});

describe("proxyPatch", () => {
  it("forwards the body and unwraps data", async () => {
    fetchMock.mockResolvedValueOnce(
      backend({ status: "success", data: { id: 7 } }),
    );

    const response = await proxyPatch("/api/items/7")(
      request("/api/items/7", { method: "PATCH", body: { name: "C" } }),
      createMockContext(),
    );

    expect(sent().init).toMatchObject({
      method: "PATCH",
      body: '{"name":"C"}',
    });
    expect(await response.json()).toMatchObject({ data: { id: 7 } });
  });
});

describe("proxyDelete", () => {
  it("resolves the endpoint builder and returns 204", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const response = await proxyDelete(
      (params) => `/api/items/${String(params.id)}`,
    )(
      request("/api/items/7", { method: "DELETE" }),
      createMockContext({ id: "7" }),
    );

    expect(sent().url).toBe("http://backend:8080/api/items/7");
    expect(sent().init.method).toBe("DELETE");
    expect(response.status).toBe(204);
  });
});

describe("proxyDelete with a body", () => {
  it("forwards the confirmation body and returns 204", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const response = await proxyDelete("/api/items/7", { withBody: true })(
      request("/api/items/7", { method: "DELETE", body: { confirm: true } }),
      createMockContext(),
    );

    expect(sent().init.body).toBe('{"confirm":true}');
    expect(response.status).toBe(204);
  });
});
