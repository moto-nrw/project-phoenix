import type { NextRequest } from "next/server";
import {
  type RouteContext,
  buildQueryString,
} from "./route-wrapper-utils.server";

/**
 * Portal-agnostic factory for the pure backend-proxy route handlers that make
 * up most of `src/app/api/`. Each portal (tenant / operator / parent) wires its
 * own base handler factories and API fetchers into {@link makeProxyFactories},
 * so the resulting `proxyGet`/`proxyPost`/… inherit that portal's auth,
 * 401-retry, and response-format behavior for free. The only logic that lives
 * here is endpoint resolution, query/body forwarding, and envelope unwrapping.
 */

type Params = Record<string, unknown>;

/** A backend endpoint: a fixed path, or a builder from route params (id-in-path). */
export type ProxyEndpoint = string | ((params: Params) => string);

interface ProxyOptions {
  /**
   * Return the backend response verbatim instead of unwrapping its
   * `{ status, data, message }` envelope. Only meaningful for the tenant
   * portal, whose fetchers return the raw envelope; operator/parent fetchers
   * already unwrap, so `raw` is a no-op there.
   */
  raw?: boolean;
}

type WiredHandler = (
  request: NextRequest,
  context: RouteContext,
) => Promise<Response>;

type NoBodyFactory = <T>(
  handler: (request: NextRequest, token: string, params: Params) => Promise<T>,
) => WiredHandler;

type WithBodyFactory = <T, B>(
  handler: (
    request: NextRequest,
    body: B,
    token: string,
    params: Params,
  ) => Promise<T>,
) => WiredHandler;

type ReadFetcher = <T>(endpoint: string, token: string) => Promise<T>;
type WriteFetcher = <T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
  idempotencyKey?: string,
) => Promise<T>;
type DeleteFetcher = (endpoint: string, token: string) => Promise<unknown>;

export interface PortalBindings {
  get: NoBodyFactory;
  post: WithBodyFactory;
  put: WithBodyFactory;
  patch?: WithBodyFactory;
  del: NoBodyFactory;
  apiGet: ReadFetcher;
  apiPost: WriteFetcher;
  apiPut: WriteFetcher;
  apiPatch?: WriteFetcher;
  apiDelete: DeleteFetcher;
  /** True when the portal's fetchers already unwrap the backend `data` envelope. */
  fetcherUnwrapsData: boolean;
}

export function makeProxyFactories(b: PortalBindings) {
  const resolve = (endpoint: ProxyEndpoint, params: Params): string =>
    typeof endpoint === "function" ? endpoint(params) : endpoint;

  const unwrap = <T>(response: unknown, raw?: boolean): T => {
    if (raw || b.fetcherUnwrapsData) return response as T;
    return (response as { data: T }).data;
  };

  /** GET proxy: forwards the query string, unwraps `data` unless `{ raw: true }`. */
  function proxyGet<T>(endpoint: ProxyEndpoint, options?: ProxyOptions) {
    return b.get<T>(async (request, token, params) => {
      const target = `${resolve(endpoint, params)}${buildQueryString(request)}`;
      return unwrap<T>(await b.apiGet<unknown>(target, token), options?.raw);
    });
  }

  /** POST proxy: forwards the parsed body, unwraps `data` unless `{ raw: true }`. */
  function proxyPost<T, B = unknown>(
    endpoint: ProxyEndpoint,
    options?: ProxyOptions,
  ) {
    return b.post<T, B>(async (request, body, token, params) => {
      const idempotencyKey = request.headers.get("Idempotency-Key");
      const target = resolve(endpoint, params);
      const response = idempotencyKey
        ? await b.apiPost<unknown, B>(target, token, body, idempotencyKey)
        : await b.apiPost<unknown, B>(target, token, body);
      return unwrap<T>(response, options?.raw);
    });
  }

  /** PUT proxy: forwards the parsed body, unwraps `data` unless `{ raw: true }`. */
  function proxyPut<T, B = unknown>(
    endpoint: ProxyEndpoint,
    options?: ProxyOptions,
  ) {
    return b.put<T, B>(async (_request, body, token, params) =>
      unwrap<T>(
        await b.apiPut<unknown, B>(resolve(endpoint, params), token, body),
        options?.raw,
      ),
    );
  }

  /** PATCH proxy: forwards the parsed body, unwraps `data` unless `{ raw: true }`. */
  function proxyPatch<T, B = unknown>(
    endpoint: ProxyEndpoint,
    options?: ProxyOptions,
  ) {
    const patch = b.patch;
    const apiPatch = b.apiPatch;
    if (!patch || !apiPatch) {
      throw new Error("proxyPatch is not configured for this portal");
    }
    return patch<T, B>(async (_request, body, token, params) =>
      unwrap<T>(
        await apiPatch<unknown, B>(resolve(endpoint, params), token, body),
        options?.raw,
      ),
    );
  }

  /** DELETE proxy: forwards no body; null result becomes 204 (per the base handler). */
  function proxyDelete(endpoint: ProxyEndpoint) {
    return b.del<null>(async (_request, token, params) => {
      await b.apiDelete(resolve(endpoint, params), token);
      return null;
    });
  }

  return { proxyGet, proxyPost, proxyPut, proxyPatch, proxyDelete };
}
