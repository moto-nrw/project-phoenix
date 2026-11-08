import { readdirSync } from "node:fs";
import path from "node:path";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

/**
 * Runs every BFF route once and watches where it reads a session. A read
 * outside the portal's response wrapper can rotate the backend refresh token
 * while Auth.js' Set-Cookie never leaves (#1938), and a renewal there cannot
 * be forced. Two observations count as such a read:
 * - Auth.js' own `auth()` is called outside its route-handler overload;
 * - a portal session helper refuses to run (assertPortalRequestContext).
 * Which import a route uses does not matter; only what it does when called.
 */

const state = vi.hoisted(() => {
  const { AsyncLocalStorage } = process.getBuiltinModule(
    "node:async_hooks",
  ) as typeof import("node:async_hooks");
  return {
    insideWrapper: new AsyncLocalStorage<true>(),
    route: "",
    findings: new Set<string>(),
  };
});

const SESSION = {
  user: { id: "1", token: "access-token", refreshToken: "r", tenantId: 7 },
  expires: "2099-01-01T00:00:00.000Z",
};

vi.mock("next-auth", () => {
  type Handler = (request: Request, context?: unknown) => Promise<Response>;
  const auth = (handler?: unknown) => {
    if (typeof handler === "function") {
      return (request: Request & { auth?: unknown }, context?: unknown) =>
        state.insideWrapper.run(true, () => {
          request.auth = SESSION;
          return (handler as Handler)(request, context);
        });
    }
    if (!state.insideWrapper.getStore()) {
      state.findings.add(`${state.route}: auth() outside its wrapper`);
    }
    return Promise.resolve(SESSION);
  };
  return {
    default: () => ({
      auth,
      handlers: { GET: vi.fn(), POST: vi.fn() },
      signIn: vi.fn(),
      unstable_update: vi.fn(),
    }),
  };
});

vi.mock("@sentry/nextjs", async (original) => ({
  ...(await original<typeof import("@sentry/nextjs")>()),
  captureException: (error: unknown) => noteGuard(error),
}));

vi.mock("~/lib/tenant-resolve.server", async (original) => ({
  ...(await original<typeof import("~/lib/tenant-resolve.server")>()),
  resolveTenantCached: async () => ({ tenant_id: 7 }),
}));

function noteGuard(error: unknown) {
  if (
    error instanceof Error &&
    error.message.includes("outside its portal's auth wrapper")
  ) {
    state.findings.add(`${state.route}: session helper outside its wrapper`);
  }
}

const apiDir = path.dirname(new URL(import.meta.url).pathname);
const METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE"] as const;

/** Every route but Auth.js' own protocol endpoints, which are its handlers. */
const routes = readdirSync(apiDir, { recursive: true, encoding: "utf8" })
  .filter((file) => path.basename(file) === "route.ts")
  .filter((file) => !file.includes("[...nextauth]"))
  .map((file) => path.dirname(file).split(path.sep).join("/"))
  .sort();

function params(route: string) {
  const values: Record<string, string | string[]> = {};
  for (const [, catchAll, name] of route.matchAll(/\[(\.\.\.)?([^\]]+)\]/g)) {
    values[name!] = catchAll ? ["1"] : "1";
  }
  return values;
}

function request(route: string, method: string) {
  const url = `http://school-a.localhost:3000/api/${route.replace(/\[[^\]]+\]/g, "1")}`;
  return new NextRequest(url, {
    method,
    headers: {
      host: "school-a.localhost:3000",
      origin: "http://school-a.localhost:3000",
      "content-type": "application/json",
    },
    ...(method === "GET" ? {} : { body: "{}" }),
  });
}

async function run(handler: unknown, route: string, method: string) {
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    const response = await Promise.race([
      (handler as (r: NextRequest, c: unknown) => Promise<Response>)(
        request(route, method),
        { params: Promise.resolve(params(route)) },
      ),
      new Promise<null>((resolve) => {
        timer = setTimeout(() => resolve(null), 2_000);
      }),
    ]);
    await response?.body?.cancel().catch(() => undefined);
  } catch (error) {
    noteGuard(error);
  } finally {
    clearTimeout(timer);
  }
}

describe("session reads of every BFF route", () => {
  beforeAll(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => Response.json({ status: "success", data: {} })),
    );
    vi.spyOn(console, "error").mockImplementation(() => {});
    vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.spyOn(console, "info").mockImplementation(() => {});
  });
  afterAll(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("finds the routes", () => {
    // Anker: the walk sees the tree, also the portals' own routes.
    expect(routes).toContain("auth/logout");
    expect(routes).toContain("traeger/auth/logout");
    expect(routes.length).toBeGreaterThan(900);
  });

  it("reads every session inside the portal's response wrapper", async () => {
    for (const route of routes) {
      state.route = route;
      const routeModule = (await import(
        path.join(apiDir, route, "route.ts")
      )) as Record<string, unknown>;
      for (const method of METHODS) {
        if (typeof routeModule[method] === "function") {
          await run(routeModule[method], route, method);
        }
      }
    }
    expect(
      [...state.findings].sort(),
      "A session read outside the portal's wrapper can rotate the refresh token while its cookie never leaves (#1938)",
    ).toEqual([]);
  }, 600_000);
});
