import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { traegerAuth } from "~/server/auth/traeger";
import { withTraegerAuth } from "~/server/auth/traeger-route";
import { incomingAnalyticsSessionHeaders } from "../analytics-session-header.server";
import { handleApiError } from "../api-helpers.server";
import {
  backendResponseError,
  isApiResponseError,
} from "../api-response-error.server";
import { makeProxyFactories } from "../route-proxy-factory.server";
import {
  createTokenExpiredResponse,
  createUnauthorizedResponse,
  extractParams,
  parseRequestBody,
  wrapInApiResponse,
} from "../route-wrapper-utils.server";

/**
 * Träger-portal route wrapper ("moto träger", #4649). Mirrors
 * lib/school/route-wrapper.server.ts, but reads the Träger NextAuth session
 * (traeger.session-token) so a Träger route can never forward a tenant,
 * parent, or school token. Reads, the holiday period writes (#4659) and the
 * Träger-Büro account routes (#4650) go through the same proxy factories.
 */

async function traegerServerFetch<T>(
  endpoint: string,
  token: string,
  options: { method: string; body?: unknown },
): Promise<T> {
  const { getServerApiUrl } = await import("~/lib/server-api-url");
  const response = await fetch(`${getServerApiUrl()}${endpoint}`, {
    method: options.method,
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
      ...(await incomingAnalyticsSessionHeaders()),
    },
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });

  if (!response.ok) {
    throw await backendResponseError(response);
  }
  if (response.status === 204) {
    return undefined as T;
  }

  const json: unknown = await response.json();
  // Unwrap the backend envelope { status, data, message } of common.Respond().
  if (
    typeof json === "object" &&
    json !== null &&
    "data" in json &&
    "status" in json
  ) {
    return (json as { data: T }).data;
  }
  return json as T;
}

/** A refreshed Träger session token, or null when there is none. */
async function refreshedToken(originalToken: string): Promise<string | null> {
  const { uncachedTraegerAuth } = await import("~/server/auth/traeger");
  const updated = await uncachedTraegerAuth();
  if (!updated?.user?.token || updated.user.token === originalToken) {
    return null;
  }
  return updated.user.token;
}

type NoBodyHandler<T> = (
  request: NextRequest,
  token: string,
  params: Record<string, unknown>,
) => Promise<T>;

type WithBodyHandler<T, B> = (
  request: NextRequest,
  body: B,
  token: string,
  params: Record<string, unknown>,
) => Promise<T>;

/**
 * Runs the request and, after a backend 401, once more with a refreshed
 * session. Only a missing refresh or a second 401 counts as an expired
 * session; any other answer of the retry (a business rejection, a null
 * DELETE result) is the answer of the request.
 */
async function executeWithRetry<T>(
  request: NextRequest,
  token: string,
  execute: (token: string) => Promise<T>,
): Promise<NextResponse> {
  try {
    return NextResponse.json(wrapInApiResponse(await execute(token)));
  } catch (error) {
    if (!isApiResponseError(error, 401)) throw error;
  }
  let fresh: string | null = null;
  try {
    fresh = await refreshedToken(token);
  } catch {
    // The refresh itself failed: the session counts as expired.
  }
  if (fresh === null) return createTokenExpiredResponse(request);
  try {
    return NextResponse.json(wrapInApiResponse(await execute(fresh)));
  } catch (retryError) {
    if (isApiResponseError(retryError, 401)) {
      return createTokenExpiredResponse(request);
    }
    throw retryError;
  }
}

function createNoBodyHandler<T>(handler: NoBodyHandler<T>) {
  return withTraegerAuth(async (request, context): Promise<NextResponse> => {
    try {
      const session = await traegerAuth();
      const token = session?.user?.token;
      if (!token) return createUnauthorizedResponse(request);
      const params = await extractParams(request, context);
      return await executeWithRetry(request, token, (current) =>
        handler(request, current, params),
      );
    } catch (error) {
      return handleApiError(error, request);
    }
  });
}

function createWithBodyHandler<T, B>(handler: WithBodyHandler<T, B>) {
  return withTraegerAuth(async (request, context): Promise<NextResponse> => {
    try {
      const session = await traegerAuth();
      const token = session?.user?.token;
      if (!token) return createUnauthorizedResponse(request);
      const params = await extractParams(request, context);
      const body = await parseRequestBody<B>(request);
      return await executeWithRetry(request, token, (current) =>
        handler(request, body, current, params),
      );
    } catch (error) {
      return handleApiError(error, request);
    }
  });
}

/**
 * Träger proxy factories: envelope-unwrapping pass-throughs on the Träger
 * session. The fetchers already unwrap `data`.
 */
export const { proxyGet, proxyPost, proxyPut, proxyDelete } =
  makeProxyFactories({
    get: createNoBodyHandler,
    post: createWithBodyHandler,
    put: createWithBodyHandler,
    del: createNoBodyHandler,
    apiGet: (endpoint, token) =>
      traegerServerFetch(endpoint, token, { method: "GET" }),
    apiPost: (endpoint, token, body) =>
      traegerServerFetch(endpoint, token, { method: "POST", body }),
    apiPut: (endpoint, token, body) =>
      traegerServerFetch(endpoint, token, { method: "PUT", body }),
    apiDelete: (endpoint, token) =>
      traegerServerFetch(endpoint, token, { method: "DELETE" }),
    fetcherUnwrapsData: true,
  });
