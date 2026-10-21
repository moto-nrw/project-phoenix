import type { Session } from "next-auth";
import { NextRequest } from "next/server";

/**
 * Shared fixtures for BFF route tests (`src/app/api/**`). Use these instead of
 * redeclaring a session type, a session, a request builder or a route context
 * per file.
 */

/** NextAuth session with the backend bearer token the route wrappers read. */
export interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

/** Authenticated tenant session whose bearer token is `test-token`. */
export const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

interface MockRequestOptions {
  /** HTTP method; defaults to GET. */
  method?: string;
  /** Sent as a JSON body with `Content-Type: application/json` when truthy. */
  body?: unknown;
  /** Appended to the URL's query string. */
  searchParams?: Record<string, string>;
}

/** Request to a route path on `http://localhost:3000`. */
export function createMockRequest(
  path: string,
  options: MockRequestOptions = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  for (const [key, value] of Object.entries(options.searchParams ?? {})) {
    url.searchParams.append(key, value);
  }

  const init: { method: string; body?: string; headers?: HeadersInit } = {
    method: options.method ?? "GET",
  };
  if (options.body) {
    init.body = JSON.stringify(options.body);
    init.headers = { "Content-Type": "application/json" };
  }

  return new NextRequest(url, init);
}

/** Route handler context with the given dynamic segment params. */
export function createMockContext(
  params: Record<string, string | string[] | undefined> = {},
) {
  return { params: Promise.resolve(params) };
}
