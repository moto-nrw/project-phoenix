import { beforeEach, describe, expect, it, vi } from "vitest";
import type { NextAuthRequest, Session } from "next-auth";
import { NextRequest } from "next/server";

// Every portal's route wrapper runs through its real response-aware Auth.js
// handler here, so a wrapper that bypasses it loses the cache policy (#4999).
vi.unmock("~/server/auth/tenant-route");
vi.unmock("~/server/auth/operator-route");
vi.unmock("~/server/auth/parent-route");
vi.unmock("~/server/auth/traeger-route");

const { mockFetch } = vi.hoisted(() => ({ mockFetch: vi.fn() }));

vi.mock("next-auth", () => {
  const session = {
    user: { id: "1", token: "access-token", tenantId: 7 },
    expires: "2099-01-01T00:00:00.000Z",
  } as unknown as Session;
  type RouteHandler = (
    request: NextAuthRequest,
    context?: unknown,
  ) => Promise<Response>;
  // Mirrors Auth.js handleAuth: the route response is copied into a new
  // Response before the session cookie is appended.
  const auth = (handler?: RouteHandler) => {
    if (!handler) return Promise.resolve(session);
    return async (request: NextAuthRequest, context?: unknown) => {
      request.auth = session;
      const response = await handler(request, context);
      const finalResponse = new Response(response.body, response);
      finalResponse.headers.append("Set-Cookie", "session=rotated; Path=/");
      return finalResponse;
    };
  };
  return {
    default: () => ({ auth, handlers: {}, signIn: vi.fn() }),
  };
});

vi.stubGlobal("fetch", mockFetch);

const PRIVATE_NO_STORE = "private, no-store";

type Route = (
  request: NextRequest,
  context: { params: Promise<Record<string, string>> },
) => Promise<Response>;

function call(route: Route, params: Record<string, string> = {}) {
  return route(new NextRequest("http://localhost:3000/api/example"), {
    params: Promise.resolve(params),
  });
}

function backendJson(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      "Content-Type":
        status >= 400 ? "application/problem+json" : "application/json",
    },
  });
}

const routes: Array<[string, () => Promise<Route>]> = [
  [
    "tenant",
    async () => (await import("~/lib/route-proxy.server")).proxyGet("/api/x"),
  ],
  [
    "tenant pass-through",
    async () =>
      (await import("~/lib/backend-proxy-route.server")).createTenantJsonProxy({
        method: "GET",
        path: "/api/x",
      }),
  ],
  [
    "operator",
    async () =>
      (await import("~/lib/operator/route-wrapper.server")).proxyGet("/api/x"),
  ],
  [
    "operator pass-through",
    async () =>
      (
        await import("~/lib/backend-proxy-route.server")
      ).createOperatorJsonProxy({ method: "GET", path: "/api/x" }),
  ],
  [
    "parent",
    async () =>
      (await import("~/lib/parent/route-wrapper.server")).proxyGet("/api/x"),
  ],
  [
    "school",
    async () =>
      (await import("~/lib/school/route-wrapper.server")).proxyGet("/api/x"),
  ],
  [
    "caterer",
    async () =>
      (await import("~/lib/caterer/route-wrapper.server")).proxyGet("/api/x"),
  ],
  [
    "traeger",
    async () =>
      (await import("~/lib/traeger/route-wrapper.server")).proxyGet("/api/x"),
  ],
  [
    "public pass-through (token routes)",
    async () =>
      (await import("~/lib/backend-proxy-route.server")).createPublicJsonProxy({
        method: "GET",
        path: "/api/x",
      }),
  ],
  [
    "operator public login step",
    async () =>
      (
        await import("~/lib/operator/route-wrapper.server")
      ).createOperatorPublicProxyPostHandler("/api/x") as Route,
  ],
];

beforeEach(() => {
  mockFetch.mockReset();
});

describe("portal route wrappers send private, no-store", () => {
  it.each(routes)("%s: JSON success", async (_portal, load) => {
    mockFetch.mockResolvedValue(
      backendJson(200, { status: "success", data: { id: 1 } }),
    );

    const response = await call(await load());

    expect(response.status).toBe(200);
    expect(response.headers.get("Cache-Control")).toBe(PRIVATE_NO_STORE);
  });

  it.each(routes)("%s: JSON error", async (_portal, load) => {
    mockFetch.mockResolvedValue(
      backendJson(404, {
        status: 404,
        title: "Not Found",
        code: "general.not_found",
      }),
    );

    const response = await call(await load());

    expect(response.status).toBe(404);
    expect(response.headers.get("Cache-Control")).toBe(PRIVATE_NO_STORE);
  });
});

describe("file and stream routes keep their own headers", () => {
  it("serves a staff avatar with its image cache policy", async () => {
    mockFetch.mockResolvedValue(
      new Response(new Uint8Array([1, 2, 3]), {
        headers: { "Content-Type": "image/png" },
      }),
    );
    const { GET } = await import("~/app/api/staff/[id]/avatar/route");

    const response = await call(GET as Route, { id: "5" });

    expect(response.headers.get("Content-Type")).toBe("image/png");
    expect(response.headers.get("Cache-Control")).toBe("private, max-age=300");
  });

  it("streams a student document with the backend's headers only", async () => {
    mockFetch.mockResolvedValue(
      new Response("%PDF-1.7", {
        headers: {
          "Content-Type": "application/pdf",
          "Content-Disposition": 'attachment; filename="a.pdf"',
        },
      }),
    );
    const { GET } =
      await import("~/app/api/students/[id]/documents/[documentId]/download/route");

    const response = await call(GET as Route, { id: "5", documentId: "9" });

    expect(response.headers.get("Content-Type")).toBe("application/pdf");
    expect(response.headers.get("Content-Disposition")).toBe(
      'attachment; filename="a.pdf"',
    );
    expect(response.headers.has("Cache-Control")).toBe(false);
  });

  it("keeps the SSE stream's no-cache policy", async () => {
    mockFetch.mockResolvedValue(
      new Response("event: ping\n\n", {
        headers: { "Content-Type": "text/event-stream" },
      }),
    );
    const { GET } = await import("~/app/api/sse/events/route");

    const response = await call(GET as Route);

    expect(response.headers.get("Content-Type")).toBe("text/event-stream");
    expect(response.headers.get("Cache-Control")).toBe("no-cache");
    await response.body?.cancel();
  });
});
