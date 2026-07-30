import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { parentAuth } from "~/server/auth/parent";
import { withParentAuth } from "~/server/auth/parent-route";
import { handleApiError } from "../api-helpers.server";
import { makeProxyFactories } from "../route-proxy-factory.server";
import {
  extractParams,
  parseRequestBody,
  wrapInApiResponse,
  createUnauthorizedResponse,
} from "../route-wrapper-utils.server";

/**
 * Parent-app route wrapper. Mirrors lib/operator/route-wrapper.ts —
 * separate so parent routes use the parent NextAuth session (parent.
 * session-token cookie) instead of the operator session. Defense-in-
 * depth against accidentally hitting tenant or operator endpoints from
 * a parent-scoped request.
 */

function is401Error(error: unknown): boolean {
  return error instanceof Error && error.message.includes("API error (401)");
}

async function tryRetryWithRefreshedToken<T>(
  originalToken: string,
  retryFn: (token: string) => Promise<T>,
): Promise<T | null> {
  const { uncachedParentAuth: uncachedAuth } =
    await import("~/server/auth/parent");
  const updatedSession = await uncachedAuth();

  if (
    !updatedSession?.user?.token ||
    updatedSession.user.token === originalToken
  ) {
    return null;
  }

  return retryFn(updatedSession.user.token);
}

async function parentServerFetch<T>(
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
    },
    body: options.body ? JSON.stringify(options.body) : undefined,
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`API error (${response.status}): ${errorText}`);
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

// Internal — used only by the proxy factories below. parentApiDelete stays
// exported because hand-rolled DELETE routes still import it directly.
function parentApiGet<T>(endpoint: string, token: string): Promise<T> {
  return parentServerFetch<T>(endpoint, token, { method: "GET" });
}

function parentApiPost<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return parentServerFetch<T>(endpoint, token, { method: "POST", body });
}

export function parentApiDelete<T>(
  endpoint: string,
  token: string,
): Promise<T> {
  return parentServerFetch<T>(endpoint, token, { method: "DELETE" });
}

// Exported for parent routes with a dynamic path segment, which the static
// proxy factories below cannot express.
export function parentApiPut<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return parentServerFetch<T>(endpoint, token, { method: "PUT", body });
}

export function parentApiPatch<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return parentServerFetch<T>(endpoint, token, { method: "PATCH", body });
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
  token: string,
  executeHandler: (token: string) => Promise<T>,
  formatResponse: (data: T) => NextResponse,
): Promise<NextResponse> {
  try {
    const data = await executeHandler(token);
    return formatResponse(data);
  } catch (handlerError) {
    if (!is401Error(handlerError)) {
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

    return NextResponse.json(
      { error: "Token expired", code: "TOKEN_EXPIRED" },
      { status: 401 },
    );
  }
}

function createParentNoBodyHandler<T>(
  handler: NoBodyHandler<T>,
  formatResponse: (data: T) => NextResponse,
) {
  return withParentAuth(async (request, context): Promise<NextResponse> => {
    try {
      const session = await parentAuth();
      if (!session?.user?.token) return createUnauthorizedResponse();
      const params = await extractParams(request, context);

      return await executeWithRetry(
        session.user.token,
        (token) => handler(request, token, params),
        formatResponse,
      );
    } catch (error) {
      return handleApiError(error);
    }
  });
}

function createParentWithBodyHandler<T, B>(handler: WithBodyHandler<T, B>) {
  return withParentAuth(async (request, context): Promise<NextResponse> => {
    try {
      const session = await parentAuth();
      if (!session?.user?.token) return createUnauthorizedResponse();
      const params = await extractParams(request, context);
      const body = await parseRequestBody<B>(request);

      return await executeWithRetry(
        session.user.token,
        (token) => handler(request, body, token, params),
        (data) => NextResponse.json(wrapInApiResponse(data)),
      );
    } catch (error) {
      return handleApiError(error);
    }
  });
}

const jsonResponse = <T>(data: T) => NextResponse.json(wrapInApiResponse(data));

function createParentGetHandler<T>(handler: NoBodyHandler<T>) {
  return createParentNoBodyHandler(handler, jsonResponse);
}

function createParentPostHandler<T, B = unknown>(
  handler: WithBodyHandler<T, B>,
) {
  return createParentWithBodyHandler(handler);
}

export function createParentDeleteHandler<T>(handler: NoBodyHandler<T>) {
  return createParentNoBodyHandler(handler, jsonResponse);
}

export function createParentPutHandler<T, B = unknown>(
  handler: WithBodyHandler<T, B>,
) {
  return createParentWithBodyHandler(handler);
}

export function createParentPatchHandler<T, B = unknown>(
  handler: WithBodyHandler<T, B>,
) {
  return createParentWithBodyHandler(handler);
}

/**
 * Parent proxy factories — envelope-wrapping pass-throughs built on the parent
 * base handlers (parent fetchers already unwrap `data`). Use these for pure
 * pass-through parent routes instead of hand-rolling
 * `createParentXHandler(async … parentApiX(…))`.
 */
export const { proxyGet, proxyPost, proxyPut } = makeProxyFactories({
  get: createParentGetHandler,
  post: createParentPostHandler,
  put: createParentPutHandler,
  del: createParentDeleteHandler,
  apiGet: parentApiGet,
  apiPost: parentApiPost,
  apiPut: parentApiPut,
  apiDelete: parentApiDelete,
  fetcherUnwrapsData: true,
});
