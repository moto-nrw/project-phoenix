import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { schoolAuth } from "~/server/auth/school";
import { withSchoolAuth } from "~/server/auth/school-route";
import { handleApiError } from "../api-helpers.server";
import { makeProxyFactories } from "../route-proxy-factory.server";
import {
  extractParams,
  parseRequestBody,
  wrapInApiResponse,
  createUnauthorizedResponse,
} from "../route-wrapper-utils.server";

/**
 * School-app route wrapper ("moto schule", #2207). Mirrors
 * lib/parent/route-wrapper.server.ts — separate so school routes use the
 * school NextAuth session (school.session-token cookie) instead of the
 * tenant or parent session. Defense-in-depth against accidentally hitting
 * tenant endpoints from a school-scoped request.
 */

function is401Error(error: unknown): boolean {
  return error instanceof Error && error.message.includes("API error (401)");
}

async function tryRetryWithRefreshedToken<T>(
  originalToken: string,
  retryFn: (token: string) => Promise<T>,
): Promise<T | null> {
  const { uncachedSchoolAuth: uncachedAuth } =
    await import("~/server/auth/school");
  const updatedSession = await uncachedAuth();

  if (
    !updatedSession?.user?.token ||
    updatedSession.user.token === originalToken
  ) {
    return null;
  }

  return retryFn(updatedSession.user.token);
}

async function schoolServerFetch<T>(
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

// Internal — used only by the proxy factories below.
function schoolApiGet<T>(endpoint: string, token: string): Promise<T> {
  return schoolServerFetch<T>(endpoint, token, { method: "GET" });
}

function schoolApiPost<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return schoolServerFetch<T>(endpoint, token, { method: "POST", body });
}

function schoolApiDelete<T>(endpoint: string, token: string): Promise<T> {
  return schoolServerFetch<T>(endpoint, token, { method: "DELETE" });
}

function schoolApiPut<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return schoolServerFetch<T>(endpoint, token, { method: "PUT", body });
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

function createSchoolNoBodyHandler<T>(
  handler: NoBodyHandler<T>,
  formatResponse: (data: T) => NextResponse,
) {
  return withSchoolAuth(async (request, context): Promise<NextResponse> => {
    try {
      const session = await schoolAuth();
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

function createSchoolWithBodyHandler<T, B>(handler: WithBodyHandler<T, B>) {
  return withSchoolAuth(async (request, context): Promise<NextResponse> => {
    try {
      const session = await schoolAuth();
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

function createSchoolGetHandler<T>(handler: NoBodyHandler<T>) {
  return createSchoolNoBodyHandler(handler, jsonResponse);
}

function createSchoolPostHandler<T, B = unknown>(
  handler: WithBodyHandler<T, B>,
) {
  return createSchoolWithBodyHandler(handler);
}

function createSchoolDeleteHandler<T>(handler: NoBodyHandler<T>) {
  return createSchoolNoBodyHandler(handler, jsonResponse);
}

function createSchoolPutHandler<T, B = unknown>(
  handler: WithBodyHandler<T, B>,
) {
  return createSchoolWithBodyHandler(handler);
}

/**
 * School proxy factories — envelope-wrapping pass-throughs built on the
 * school base handlers (school fetchers already unwrap `data`). Use these
 * for pure pass-through school routes. Only proxyGet is exported so far —
 * the portal is read-only today; export more verbs as routes need them.
 */
export const { proxyGet } = makeProxyFactories({
  get: createSchoolGetHandler,
  post: createSchoolPostHandler,
  put: createSchoolPutHandler,
  del: createSchoolDeleteHandler,
  apiGet: schoolApiGet,
  apiPost: schoolApiPost,
  apiPut: schoolApiPut,
  apiDelete: schoolApiDelete,
  fetcherUnwrapsData: true,
});
