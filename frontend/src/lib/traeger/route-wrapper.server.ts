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
 * parent, or school token. Reads and the holiday period writes (#4659) go
 * through the same proxy factories.
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

const traegerApiGet = <T>(endpoint: string, token: string) =>
  traegerServerFetch<T>(endpoint, token, { method: "GET" });
const traegerApiPost = <T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
) => traegerServerFetch<T>(endpoint, token, { method: "POST", body });
const traegerApiPut = <T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
) => traegerServerFetch<T>(endpoint, token, { method: "PUT", body });
const traegerApiDelete = <T>(endpoint: string, token: string) =>
  traegerServerFetch<T>(endpoint, token, { method: "DELETE" });

/** Retries once with a refreshed Träger session after a backend 401. */
async function retryWithRefreshedToken<T>(
  originalToken: string,
  retry: (token: string) => Promise<T>,
): Promise<T | null> {
  const { uncachedTraegerAuth } = await import("~/server/auth/traeger");
  const updated = await uncachedTraegerAuth();
  if (!updated?.user?.token || updated.user.token === originalToken) {
    return null;
  }
  return retry(updated.user.token);
}

async function executeWithRetry<T>(
  request: NextRequest,
  token: string,
  execute: (token: string) => Promise<T>,
): Promise<NextResponse> {
  try {
    return NextResponse.json(wrapInApiResponse(await execute(token)));
  } catch (error) {
    if (!isApiResponseError(error, 401)) throw error;
    let retried: T | null;
    try {
      retried = await retryWithRefreshedToken(token, execute);
    } catch (retryError) {
      // Only a second 401 means the session is gone; any other refusal of
      // the retried request (a 400 or 409 of the holiday period) is the answer.
      if (!isApiResponseError(retryError, 401)) throw retryError;
      retried = null;
    }
    if (retried !== null) {
      return NextResponse.json(wrapInApiResponse(retried));
    }
    return createTokenExpiredResponse(request);
  }
}

type Params = Record<string, unknown>;

function createTraegerNoBodyHandler<T>(
  handler: (request: NextRequest, token: string, params: Params) => Promise<T>,
) {
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

function createTraegerWithBodyHandler<T, B>(
  handler: (
    request: NextRequest,
    body: B,
    token: string,
    params: Params,
  ) => Promise<T>,
) {
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

/** Proxy factories of the Träger portal: forward the query or body, unwrap `data`. */
export const { proxyGet, proxyPost, proxyPut } = makeProxyFactories({
  get: createTraegerNoBodyHandler,
  post: createTraegerWithBodyHandler,
  put: createTraegerWithBodyHandler,
  del: createTraegerNoBodyHandler,
  apiGet: traegerApiGet,
  apiPost: traegerApiPost,
  apiPut: traegerApiPut,
  apiDelete: traegerApiDelete,
  fetcherUnwrapsData: true,
});
