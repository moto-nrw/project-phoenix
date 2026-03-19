import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { handleApiError } from "../api-helpers";
import {
  type RouteContext,
  buildQueryString,
  extractParams,
  parseRequestBody,
  wrapInApiResponse,
  createUnauthorizedResponse,
} from "../route-wrapper-utils";
/**
 * Checks if error is a 401 authentication error
 */
function is401Error(error: unknown): boolean {
  return error instanceof Error && error.message.includes("API error (401)");
}

/**
 * Attempts to retry a request with a refreshed token.
 * NextAuth handles token refresh transparently via JWT callback —
 * calling auth() again may return a fresh token.
 */
async function tryRetryWithRefreshedToken<T>(
  originalToken: string,
  retryFn: (token: string) => Promise<T>,
): Promise<T | null> {
  const { auth } = await import("~/server/auth");
  const updatedSession = await auth();

  if (
    !updatedSession?.user?.token ||
    updatedSession.user.token === originalToken
  ) {
    return null;
  }

  return retryFn(updatedSession.user.token);
}

async function operatorServerFetch<T>(
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

export function operatorApiGet<T>(endpoint: string, token: string): Promise<T> {
  return operatorServerFetch<T>(endpoint, token, { method: "GET" });
}

export function operatorApiPost<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return operatorServerFetch<T>(endpoint, token, { method: "POST", body });
}

export function operatorApiPut<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return operatorServerFetch<T>(endpoint, token, { method: "PUT", body });
}

export function operatorApiDelete<T>(
  endpoint: string,
  token: string,
): Promise<T> {
  return operatorServerFetch<T>(endpoint, token, { method: "DELETE" });
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
 * Executes handler with retry logic on 401 errors.
 * Mirrors the teacher route-wrapper pattern.
 */
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

function createOperatorNoBodyHandler<T>(
  handler: NoBodyHandler<T>,
  formatResponse: (data: T) => NextResponse,
) {
  return async (
    request: NextRequest,
    context: RouteContext,
  ): Promise<NextResponse> => {
    try {
      const { auth } = await import("~/server/auth");
      const session = await auth();
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
  };
}

function createOperatorWithBodyHandler<T, B>(handler: WithBodyHandler<T, B>) {
  return async (
    request: NextRequest,
    context: RouteContext,
  ): Promise<NextResponse> => {
    try {
      const { auth } = await import("~/server/auth");
      const session = await auth();
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
  };
}

const jsonResponse = <T>(data: T) => NextResponse.json(wrapInApiResponse(data));

export function createOperatorGetHandler<T>(handler: NoBodyHandler<T>) {
  return createOperatorNoBodyHandler(handler, jsonResponse);
}

export function createOperatorPostHandler<T, B = unknown>(
  handler: WithBodyHandler<T, B>,
) {
  return createOperatorWithBodyHandler(handler);
}

export function createOperatorPutHandler<T, B = unknown>(
  handler: WithBodyHandler<T, B>,
) {
  return createOperatorWithBodyHandler(handler);
}

export function createOperatorDeleteHandler<T>(handler: NoBodyHandler<T>) {
  return createOperatorNoBodyHandler(handler, (data: T) => {
    if (data === null || data === undefined) {
      return new NextResponse(null, { status: 204 });
    }
    return NextResponse.json(wrapInApiResponse(data));
  });
}

export function createOperatorProxyGetHandler<T>(backendEndpoint: string) {
  return createOperatorGetHandler<T>(
    async (request: NextRequest, token: string) => {
      const endpoint = `${backendEndpoint}${buildQueryString(request)}`;
      return operatorApiGet<T>(endpoint, token);
    },
  );
}

export { isStringParam } from "../route-wrapper-utils";
