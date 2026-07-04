import type { NextRequest, NextResponse } from "next/server";
import {
  buildQueryString,
  type RouteContext,
} from "./route-wrapper-utils.server";

type Params = Record<string, unknown>;

export type ProxyEndpoint = string | ((params: Params) => string);

interface ProxyOptions {
  raw?: boolean;
}

type WiredHandler = (
  request: NextRequest,
  context: RouteContext,
) => Promise<NextResponse>;

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
  fetcherUnwrapsData: boolean;
}

export function makeProxyFactories(b: PortalBindings) {
  const resolve = (endpoint: ProxyEndpoint, params: Params): string =>
    typeof endpoint === "function" ? endpoint(params) : endpoint;

  const unwrap = <T>(response: unknown, raw?: boolean): T => {
    if (raw || b.fetcherUnwrapsData) return response as T;
    return (response as { data: T }).data;
  };

  function proxyGet<T>(endpoint: ProxyEndpoint, options?: ProxyOptions) {
    return b.get<T>(async (request, token, params) => {
      const target = `${resolve(endpoint, params)}${buildQueryString(request)}`;
      return unwrap<T>(await b.apiGet<unknown>(target, token), options?.raw);
    });
  }

  function proxyPost<T, B = unknown>(
    endpoint: ProxyEndpoint,
    options?: ProxyOptions,
  ) {
    return b.post<T, B>(async (_request, body, token, params) =>
      unwrap<T>(
        await b.apiPost<unknown, B>(resolve(endpoint, params), token, body),
        options?.raw,
      ),
    );
  }

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

  function proxyDelete(endpoint: ProxyEndpoint) {
    return b.del<null>(async (_request, token, params) => {
      await b.apiDelete(resolve(endpoint, params), token);
      return null;
    });
  }

  return { proxyGet, proxyPost, proxyPut, proxyPatch, proxyDelete };
}
