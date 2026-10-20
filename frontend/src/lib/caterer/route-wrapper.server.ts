import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { catererAuth } from "~/server/auth/caterer";
import { withCatererAuth } from "~/server/auth/caterer-route";
import { incomingAnalyticsSessionHeaders } from "../analytics-session-header.server";
import { handleApiError } from "../api-helpers.server";
import {
  backendResponseError,
  isApiResponseError,
} from "../api-response-error.server";
import { makeProxyFactories } from "../route-proxy-factory.server";
import {
  extractParams,
  parseRequestBody,
  wrapInApiResponse,
  createTokenExpiredResponse,
  createUnauthorizedResponse,
} from "../route-wrapper-utils.server";

/**
 * Caterer-portal route wrapper (ADR 0075). Mirrors
 * lib/school/route-wrapper.server.ts — separate so caterer routes use the
 * caterer NextAuth session (caterer.session-token cookie) and never a
 * tenant, parent or school session.
 */

async function tryRetryWithRefreshedToken<T>(
  originalToken: string,
  retryFn: (token: string) => Promise<T>,
): Promise<T | null> {
  const { uncachedCatererAuth: uncachedAuth } =
    await import("~/server/auth/caterer");
  const updatedSession = await uncachedAuth();

  if (
    !updatedSession?.user?.token ||
    updatedSession.user.token === originalToken
  ) {
    return null;
  }

  return retryFn(updatedSession.user.token);
}

async function catererServerFetch<T>(
  endpoint: string,
  token: string,
  options: { method: string; body?: unknown },
): Promise<T> {
  const { getServerApiUrl } = await import("~/lib/server-api-url");
  const url = `${getServerApiUrl()}${endpoint}`;

  const response = await fetch(url, {
    method: options.method,
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
      ...(await incomingAnalyticsSessionHeaders()),
    },
    body: options.body ? JSON.stringify(options.body) : undefined,
  });

  if (!response.ok) {
    throw await backendResponseError(response);
  }

  return parseResponse<T>(response);
}

function parseResponse<T>(response: Response): Promise<T> {
  if (response.status === 204) {
    return Promise.resolve(undefined as T);
  }

  return response.json().then((json: unknown) => {
    // Unwrap backend envelope { status, data, message } from common.Respond()
    if (
      typeof json === "object" &&
      json !== null &&
      "data" in json &&
      "status" in json
    ) {
      return (json as { data: T }).data;
    }
    return json as T;
  });
}

// Internal — used only by the proxy factories below.
function catererApiGet<T>(endpoint: string, token: string): Promise<T> {
  return catererServerFetch<T>(endpoint, token, { method: "GET" });
}

function catererApiPost<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return catererServerFetch<T>(endpoint, token, { method: "POST", body });
}

function catererApiDelete<T>(endpoint: string, token: string): Promise<T> {
  return catererServerFetch<T>(endpoint, token, { method: "DELETE" });
}

function catererApiPut<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return catererServerFetch<T>(endpoint, token, { method: "PUT", body });
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

async function executeWithRetry<T>(
  request: NextRequest,
  token: string,
  executeHandler: (token: string) => Promise<T>,
  formatResponse: (data: T) => NextResponse,
): Promise<NextResponse> {
  try {
    const data = await executeHandler(token);
    return formatResponse(data);
  } catch (handlerError) {
    if (!isApiResponseError(handlerError, 401)) {
      throw handlerError;
    }

    try {
      const retryData = await tryRetryWithRefreshedToken(token, executeHandler);
      if (retryData !== null) {
        return formatResponse(retryData);
      }
    } catch {
      // Retry failed, fall through to token expired
    }

    return createTokenExpiredResponse(request);
  }
}

function createCatererNoBodyHandler<T>(
  handler: NoBodyHandler<T>,
  formatResponse: (data: T) => NextResponse,
) {
  return withCatererAuth(async (request, context): Promise<NextResponse> => {
    try {
      const session = await catererAuth();
      if (!session?.user?.token) return createUnauthorizedResponse(request);
      const params = await extractParams(request, context);

      return await executeWithRetry(
        request,
        session.user.token,
        (token) => handler(request, token, params),
        formatResponse,
      );
    } catch (error) {
      return handleApiError(error, request);
    }
  });
}

function createCatererWithBodyHandler<T, B>(handler: WithBodyHandler<T, B>) {
  return withCatererAuth(async (request, context): Promise<NextResponse> => {
    try {
      const session = await catererAuth();
      if (!session?.user?.token) return createUnauthorizedResponse(request);
      const params = await extractParams(request, context);
      const body = await parseRequestBody<B>(request);

      return await executeWithRetry(
        request,
        session.user.token,
        (token) => handler(request, body, token, params),
        (data) => NextResponse.json(wrapInApiResponse(data)),
      );
    } catch (error) {
      return handleApiError(error, request);
    }
  });
}

const jsonResponse = <T>(data: T) => NextResponse.json(wrapInApiResponse(data));

function createCatererGetHandler<T>(handler: NoBodyHandler<T>) {
  return createCatererNoBodyHandler(handler, jsonResponse);
}

function createCatererPostHandler<T, B = unknown>(
  handler: WithBodyHandler<T, B>,
) {
  return createCatererWithBodyHandler(handler);
}

function createCatererDeleteHandler<T>(handler: NoBodyHandler<T>) {
  return createCatererNoBodyHandler(handler, jsonResponse);
}

function createCatererPutHandler<T, B = unknown>(
  handler: WithBodyHandler<T, B>,
) {
  return createCatererWithBodyHandler(handler);
}

/**
 * Caterer proxy factories — envelope-wrapping pass-throughs built on the
 * caterer base handlers (caterer fetchers already unwrap `data`). Use these
 * for pure pass-through caterer routes. The caterer reads its schools'
 * orders and confirms them, so GET and POST are exported.
 */
export const { proxyGet, proxyPost } = makeProxyFactories({
  get: createCatererGetHandler,
  post: createCatererPostHandler,
  put: createCatererPutHandler,
  del: createCatererDeleteHandler,
  apiGet: catererApiGet,
  apiPost: catererApiPost,
  apiPut: catererApiPut,
  apiDelete: catererApiDelete,
  fetcherUnwrapsData: true,
});
