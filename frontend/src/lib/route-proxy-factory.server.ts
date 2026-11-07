import type { NextRequest } from "next/server";
import {
  type RouteContext,
  allowedQueryString,
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

interface GetProxyOptions extends ProxyOptions {
  /**
   * Forward only these query parameters (first non-empty value each) instead
   * of the whole query string, for endpoints whose backend audits or bounds
   * what it is asked.
   */
  forwardQuery?: readonly string[];
}

interface PostProxyOptions extends ProxyOptions {
  /** Reject a larger request body with 413 while reading it. */
  maxBodyBytes?: number;
}

interface DeleteProxyOptions {
  /** Forward the JSON request body, e.g. a confirmation payload. */
  withBody?: boolean;
}

type WiredHandler = (
  request: NextRequest,
  context: RouteContext,
) => Promise<Response>;

type NoBodyFactory = <T>(
  handler: (request: NextRequest, token: string, params: Params) => Promise<T>,
) => WiredHandler;

type BodyHandler<T, B> = (
  request: NextRequest,
  body: B,
  token: string,
  params: Params,
) => Promise<T>;

type WithBodyFactory = <T, B>(handler: BodyHandler<T, B>) => WiredHandler;

type BoundedBodyFactory = <T, B>(
  handler: BodyHandler<T, B>,
  options: { maxBodyBytes: number },
) => WiredHandler;

type ReadFetcher = <T>(endpoint: string, token: string) => Promise<T>;
type WriteFetcher = <T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
  idempotencyKey?: string,
) => Promise<T>;
type DeleteFetcher = (endpoint: string, token: string) => Promise<unknown>;
type DeleteWithBodyFetcher = (
  endpoint: string,
  token: string,
  body: unknown,
) => Promise<unknown>;

interface PortalBindings {
  get: NoBodyFactory;
  post: WithBodyFactory;
  put: WithBodyFactory;
  patch?: WithBodyFactory;
  del: NoBodyFactory;
  /** Portals that bound request bodies while reading them. */
  boundedPost?: BoundedBodyFactory;
  /** Portals with DELETE routes that carry a JSON body. */
  delWithBody?: WithBodyFactory;
  apiGet: ReadFetcher;
  apiPost: WriteFetcher;
  apiPut: WriteFetcher;
  apiPatch?: WriteFetcher;
  apiDelete: DeleteFetcher;
  apiDeleteWithBody?: DeleteWithBodyFetcher;
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

  const query = (request: NextRequest, allowed?: readonly string[]): string =>
    allowed ? allowedQueryString(request, allowed) : buildQueryString(request);

  /** GET proxy: forwards the query string, unwraps `data` unless `{ raw: true }`. */
  function proxyGet<T>(endpoint: ProxyEndpoint, options?: GetProxyOptions) {
    return b.get<T>(async (request, token, params) => {
      const target = `${resolve(endpoint, params)}${query(request, options?.forwardQuery)}`;
      return unwrap<T>(await b.apiGet<unknown>(target, token), options?.raw);
    });
  }

  /** POST proxy: forwards the parsed body, unwraps `data` unless `{ raw: true }`. */
  function proxyPost<T, B = unknown>(
    endpoint: ProxyEndpoint,
    options?: PostProxyOptions,
  ) {
    const handler: BodyHandler<T, B> = async (request, body, token, params) => {
      const idempotencyKey = request.headers.get("Idempotency-Key");
      const target = resolve(endpoint, params);
      const response = idempotencyKey
        ? await b.apiPost<unknown, B>(target, token, body, idempotencyKey)
        : await b.apiPost<unknown, B>(target, token, body);
      return unwrap<T>(response, options?.raw);
    };
    if (options?.maxBodyBytes === undefined) return b.post<T, B>(handler);
    if (!b.boundedPost) {
      throw new Error("maxBodyBytes is not supported for this portal");
    }
    return b.boundedPost<T, B>(handler, {
      maxBodyBytes: options.maxBodyBytes,
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

  /** DELETE proxy: null result becomes 204 (per the base handler). */
  function proxyDelete(endpoint: ProxyEndpoint, options?: DeleteProxyOptions) {
    if (!options?.withBody) {
      return b.del<null>(async (_request, token, params) => {
        await b.apiDelete(resolve(endpoint, params), token);
        return null;
      });
    }
    const delWithBody = b.delWithBody;
    const apiDeleteWithBody = b.apiDeleteWithBody;
    if (!delWithBody || !apiDeleteWithBody) {
      throw new Error("DELETE with body is not configured for this portal");
    }
    return delWithBody<null, unknown>(async (_request, body, token, params) => {
      await apiDeleteWithBody(resolve(endpoint, params), token, body);
      return null;
    });
  }

  return { proxyGet, proxyPost, proxyPut, proxyPatch, proxyDelete };
}
