import type { NextRequest } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { withPrivateNoStore } from "~/lib/private-no-store";
import { createLogger } from "~/lib/logger";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";
import {
  clientIdentityHeaders,
  getClientForwardHeaders,
} from "~/lib/client-headers.server";
import { operatorAuth, uncachedOperatorAuth } from "~/server/auth/operator";
import { withOperatorAuth } from "~/server/auth/operator-route";
import { handleApiError } from "~/lib/api-helpers.server";
import {
  ApiResponseError,
  problemResponse,
} from "~/lib/api-response-error.server";
import { captureBffException } from "~/lib/sentry-bff.server";
import {
  sendWithTokenRefresh,
  type FreshSessionReader,
} from "~/lib/token-refresh-retry.server";

type JsonMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

export interface JsonProxyOptions {
  method: JsonMethod;
  path:
    | string
    | ((
        request: NextRequest,
        params: Record<string, string | string[] | undefined>,
      ) => string | Response);
  forwardQuery?: boolean | readonly string[];
  cache?: RequestCache;
  contentTypeOnGet?: boolean;
  explicitGetMethod?: boolean;
  contentTypeOnDelete?: boolean;
  unauthorizedError?: string;
  body?: "json" | "optional-json" | "form" | "none";
  networkErrorMessage?: string;
  networkErrorFromException?: boolean;
  /**
   * The browser's client IP, User-Agent and analytics session always travel
   * (`clientIdentityHeaders`): without them the backend rate-limits and
   * audits the frontend container, so all visitors share one bucket. `true`
   * adds the full `getClientForwardHeaders` set (frontend origin, User-Agent
   * fallback); `false` sends none of them.
   */
  forwardClientHeaders?: boolean;
  additionalHeaders?: (request: NextRequest) => Record<string, string>;
  mapJsonBody?: (body: unknown) => unknown;
  invalidJsonMessage?: string;
  invalidJsonResponse?: (request: NextRequest) => Response;
  networkErrorResponse?: (error: unknown, request: NextRequest) => Response;
  includeEmptyBody?: boolean;
  /**
   * Repeat a backend 401 once with the refreshed session. Opt-in: a route
   * that answers a refused credential (current password) with 401 must not
   * send that credential twice.
   */
  retryOn401?: boolean;
  onSuccess?: (response: Response) => Response | Promise<Response>;
  onBackendError?: (response: Response) => void;
}

const logger = createLogger({ component: "JsonBackendProxy" });

/**
 * Execute transparent JSON proxy routes through one fetch/error-response path.
 * With `readFreshSession`, a backend 401 repeats the request once with the
 * refreshed session's token. Exported for the portal wrappers that bind their
 * own session (caterer) and for session transitions, which pass no reader.
 */
export async function proxyJsonRequest(
  request: NextRequest,
  context: RouteContext | undefined,
  options: JsonProxyOptions,
  token?: string,
  readFreshSession?: FreshSessionReader,
): Promise<Response> {
  const {
    method,
    path,
    forwardQuery = false,
    cache,
    contentTypeOnGet = true,
    explicitGetMethod = false,
    contentTypeOnDelete = true,
    body = method === "GET" || method === "DELETE" ? "none" : "json",
    onSuccess,
    onBackendError,
    networkErrorMessage = "Internal Server Error",
    networkErrorFromException = false,
    forwardClientHeaders,
    additionalHeaders,
    mapJsonBody,
    invalidJsonMessage,
    invalidJsonResponse,
    networkErrorResponse,
    includeEmptyBody = false,
  } = options;

  try {
    const resolvedPath =
      typeof path === "string"
        ? path
        : path(request, (await context?.params) ?? {});
    if (resolvedPath instanceof Response) return resolvedPath;
    const url = new URL(resolvedPath, getServerApiUrl());
    if (forwardQuery) {
      request.nextUrl.searchParams.forEach((value, key) => {
        if (forwardQuery === true || forwardQuery.includes(key)) {
          url.searchParams.append(key, value);
        }
      });
    }
    let requestBody: BodyInit | undefined;
    if (body === "json") {
      let parsed: unknown;
      try {
        parsed = await request.json();
      } catch {
        if (invalidJsonResponse) return invalidJsonResponse(request);
        if (invalidJsonMessage) {
          return problemResponse(400, invalidJsonMessage, request);
        }
        return problemResponse(400, "Invalid JSON", request);
      }
      requestBody = JSON.stringify(mapJsonBody ? mapJsonBody(parsed) : parsed);
    } else if (body === "optional-json") {
      const parsed = await request.json().catch(() => ({}));
      requestBody = JSON.stringify(parsed);
    } else if (body === "form") {
      requestBody = await request.formData();
    }
    const headers: Record<string, string> = {
      ...(additionalHeaders ? additionalHeaders(request) : {}),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(forwardClientHeaders === false
        ? {}
        : forwardClientHeaders
          ? getClientForwardHeaders(request)
          : clientIdentityHeaders(request.headers)),
      ...((
        body === "form"
          ? false
          : method === "GET"
            ? contentTypeOnGet
            : method === "DELETE"
              ? contentTypeOnDelete
              : true
      )
        ? { "Content-Type": "application/json" }
        : {}),
    };
    const init: RequestInit = {
      ...(method === "GET" && !explicitGetMethod ? {} : { method }),
      ...(Object.keys(headers).length ? { headers } : {}),
      ...(cache ? { cache } : {}),
      ...(requestBody === undefined && !includeEmptyBody
        ? {}
        : { body: requestBody }),
    };
    const send = (bearer?: string) => {
      if (bearer && init.headers) {
        (init.headers as Record<string, string>).Authorization =
          `Bearer ${bearer}`;
      }
      return Object.keys(init).length
        ? fetch(url.toString(), init)
        : fetch(url.toString());
    };
    const response =
      token && readFreshSession
        ? await sendWithTokenRefresh(token, send, readFreshSession)
        : await send(token);
    if (!response.ok) {
      try {
        onBackendError?.(response);
      } catch {
        // Observability must not replace a backend response.
      }
      return forwardBackendResponse(response, request);
    }
    return onSuccess
      ? onSuccess(response)
      : forwardBackendResponse(response, request);
  } catch (error) {
    captureBffException(error, request);
    logger.error("JSON backend proxy failed", {
      method,
      path: typeof path === "string" ? path : request.nextUrl.pathname,
      error: error instanceof Error ? error.message : String(error),
    });
    if (networkErrorResponse) return networkErrorResponse(error, request);
    return problemResponse(
      500,
      networkErrorFromException && error instanceof Error
        ? error.message
        : networkErrorMessage,
      request,
    );
  }
}

/** Tenant-session JSON proxy. Never accepts a caller-supplied bearer token. */
export function createTenantJsonProxy(options: JsonProxyOptions) {
  return withTenantAuth(async (request: NextRequest, context: RouteContext) => {
    const session = await auth();
    if (!session?.user?.token) {
      return problemResponse(
        401,
        options.unauthorizedError ?? "Unauthorized",
        request,
      );
    }
    return proxyJsonRequest(
      request,
      context,
      options,
      session.user.token,
      options.retryOn401 ? () => uncachedAuth() : undefined,
    );
  });
}

/** Registration-like routes where a tenant session is optional but meaningful. */
export function createOptionalTenantJsonProxy(options: JsonProxyOptions) {
  return withTenantAuth(async (request: NextRequest, context: RouteContext) => {
    const session = await auth();
    return proxyJsonRequest(request, context, options, session?.user?.token);
  });
}

/** Public JSON proxy; no tenant session or Authorization header is attached. */
export function createPublicJsonProxy(options: JsonProxyOptions) {
  // No auth handler applies the cache policy here, and token routes
  // (enrollment edit links) answer with personal data.
  return withPrivateNoStore((request: NextRequest, context?: RouteContext) =>
    proxyJsonRequest(request, context, options),
  );
}

/** Operator-session JSON proxy with the operator's own refresh boundary. */
export function createOperatorJsonProxy(options: JsonProxyOptions) {
  return withOperatorAuth(
    async (request: NextRequest, context: RouteContext) => {
      const session = await operatorAuth();
      if (!session?.user?.token) {
        return problemResponse(401, "Unauthorized", request);
      }
      return proxyJsonRequest(
        request,
        context,
        options,
        session.user.token,
        () => uncachedOperatorAuth(),
      );
    },
  );
}

/** Adapt api* helper-backed routes that have a deliberate success envelope. */
export function createTenantApiAdapter(
  handler: (
    request: NextRequest,
    token: string,
    context: RouteContext | undefined,
  ) => Promise<Response>,
  onLocalError?: (error: unknown, request: NextRequest) => Response,
) {
  return withTenantAuth(async (request: NextRequest, context: RouteContext) => {
    try {
      const session = await auth();
      if (!session?.user?.token) {
        return problemResponse(401, "Unauthorized", request);
      }
      return await handler(request, session.user.token, context);
    } catch (error) {
      if (error instanceof ApiResponseError || !onLocalError) {
        return handleApiError(error, request);
      }
      captureBffException(error, request);
      return onLocalError(error, request);
    }
  });
}
