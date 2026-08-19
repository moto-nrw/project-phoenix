import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { withOperatorAuth } from "~/server/auth/operator-route";
import { handleApiError } from "../api-helpers.server";
import { recordBackendProxyMetric } from "../backend-proxy-metrics";
import { sanitizeEndpoint } from "../log-sanitize";
import { makeProxyFactories } from "../route-proxy-factory.server";
import {
  extractParams,
  parseRequestBody,
  wrapInApiResponse,
  createUnauthorizedResponse,
} from "../route-wrapper-utils.server";
/**
 * Checks if error is a 401 authentication error
 */
function is401Error(error: unknown): boolean {
  return error instanceof Error && error.message.includes("API error (401)");
}

async function readOperatorSession() {
  const { operatorAuth } = await import("~/server/auth/operator");
  return operatorAuth();
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
  const { uncachedOperatorAuth: uncachedAuth } =
    await import("~/server/auth/operator");
  const updatedSession = await uncachedAuth();

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
  const startedAt = Date.now();
  let responseStatus = 0;
  let outcome: "success" | "backend_error" | "network_error" = "success";

  try {
    const response = await fetch(url, {
      method: options.method,
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body: options.body ? JSON.stringify(options.body) : undefined,
    });
    responseStatus = response.status;

    if (!response.ok) {
      outcome = "backend_error";
      const errorText = await response.text();
      throw new Error(`API error (${response.status}): ${errorText}`);
    }

    return parseResponse<T>(response);
  } catch (error) {
    if (outcome === "success") {
      outcome = "network_error";
    }
    throw error;
  } finally {
    await recordOperatorBackendProxyMetric({
      method: options.method,
      endpoint,
      status: responseStatus,
      durationMs: Date.now() - startedAt,
      outcome,
    });
  }
}

async function recordOperatorBackendProxyMetric(args: {
  method: string;
  endpoint: string;
  status: number;
  durationMs: number;
  outcome: string;
}) {
  try {
    recordBackendProxyMetric({
      method: args.method,
      backendEndpoint: sanitizeEndpoint(args.endpoint),
      status: args.status,
      durationMs: args.durationMs,
      outcome: args.outcome,
      scope: "operator",
    });
  } catch {
    // Metrics must never alter request behavior.
  }
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
  return withOperatorAuth(async (request, context): Promise<NextResponse> => {
    try {
      const session = await readOperatorSession();
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

function createOperatorWithBodyHandler<T, B>(handler: WithBodyHandler<T, B>) {
  return withOperatorAuth(async (request, context): Promise<NextResponse> => {
    try {
      const session = await readOperatorSession();
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

/**
 * Forwards a backend Response to a NextResponse verbatim:
 * - 204 → empty-body NextResponse with status 204
 * - JSON content-type → pass-through with original status
 * - Non-JSON → { message } envelope (429 gets a friendly German message)
 *
 * Shared by all proxy helpers below so their response-forwarding behavior
 * stays identical.
 *
 * Error-key convention: the operator backend's ErrResponse struct serializes
 * with `json:"message"` (backend/api/operator/middleware.go), so operator API
 * errors natively ship as { status, message } — distinct from the main/tenant
 * backend which uses `json:"error"`. This helper keeps the operator-native
 * `message` key intact on both forwarded JSON and the synthetic non-JSON
 * envelope so that proxy output stays consistent with what the operator
 * backend itself emits. Operator-side consumers should dual-read
 * `data.message ?? data.error` (see frontend/src/lib/operator/api-helpers.ts)
 * to stay robust against any intermediate layer that normalizes to `error`.
 */
async function forwardBackendResponse(
  response: Response,
): Promise<NextResponse> {
  if (response.status === 204) {
    return new NextResponse(null, { status: 204 });
  }

  const contentType = response.headers.get("content-type");
  if (!contentType?.includes("application/json")) {
    const message =
      response.status === 429
        ? "Zu viele Anfragen. Bitte versuchen Sie es später erneut."
        : (await response.text()) || response.statusText;
    return NextResponse.json({ message }, { status: response.status });
  }

  const data: unknown = await response.json();
  return NextResponse.json(data, { status: response.status });
}

/**
 * Creates a POST route handler that proxies the backend response verbatim
 * (preserving status codes and error messages) while adding 401 retry logic.
 * Used for POST routes that need verbatim status/error forwarding with a JSON body.
 */
export function createOperatorProxyPostHandler(backendEndpoint: string) {
  return withOperatorAuth(async (request): Promise<NextResponse> => {
    try {
      const session = await readOperatorSession();
      if (!session?.user?.token) return createUnauthorizedResponse();

      let body: unknown;
      try {
        body = await parseRequestBody(request);
      } catch {
        return NextResponse.json(
          { message: "Ungültige Anfrage" },
          { status: 400 },
        );
      }
      const { getServerApiUrl } = await import("~/lib/server-api-url");
      const { getClientForwardHeaders } =
        await import("~/lib/client-headers.server");
      const url = `${getServerApiUrl()}${backendEndpoint}`;
      const forwardHeaders = getClientForwardHeaders(request);
      const startedAt = Date.now();

      const makeRequest = async (token: string) =>
        fetch(url, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
            ...forwardHeaders,
          },
          body: JSON.stringify(body),
        });

      let response = await makeRequest(session.user.token);

      if (response.status === 401) {
        const retried = await tryRetryWithRefreshedToken(
          session.user.token,
          async (newToken) => makeRequest(newToken),
        );
        if (retried) response = retried;
      }

      await recordOperatorBackendProxyMetric({
        method: "POST",
        endpoint: backendEndpoint,
        status: response.status,
        durationMs: Date.now() - startedAt,
        outcome: response.ok ? "success" : "backend_error",
      });
      return forwardBackendResponse(response);
    } catch (error) {
      return handleApiError(error);
    }
  });
}

/**
 * Creates an UNAUTHENTICATED POST route handler that parses a JSON body and
 * proxies it to a backend endpoint. For public invitation flows where the
 * token is carried in the request body, not the session.
 *
 * On parse failure: 400 with a German "Ungültige Anfrage" message.
 * On fetch failure: 500 with a generic German internal-error message
 *                   (no error detail to prevent information leakage on public routes).
 */
interface OperatorPublicProxyOptions {
  transformBody?: (
    request: NextRequest,
    body: unknown,
  ) => unknown | Promise<unknown>;
  decorateResponse?: (
    request: NextRequest,
    response: NextResponse,
    backendSucceeded: boolean,
  ) => void | Promise<void>;
}

export function createOperatorPublicProxyPostHandler(
  backendEndpoint: string,
  options: OperatorPublicProxyOptions = {},
) {
  return async (request: NextRequest): Promise<NextResponse> => {
    let body: unknown;
    try {
      body = await parseRequestBody(request);
      body = (await options.transformBody?.(request, body)) ?? body;
    } catch {
      return NextResponse.json(
        { message: "Ungültige Anfrage" },
        { status: 400 },
      );
    }

    try {
      const { getServerApiUrl } = await import("~/lib/server-api-url");
      const { getClientForwardHeaders } =
        await import("~/lib/client-headers.server");
      const startedAt = Date.now();
      const response = await fetch(`${getServerApiUrl()}${backendEndpoint}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...getClientForwardHeaders(request),
        },
        body: JSON.stringify(body),
      });

      await recordOperatorBackendProxyMetric({
        method: "POST",
        endpoint: backendEndpoint,
        status: response.status,
        durationMs: Date.now() - startedAt,
        outcome: response.ok ? "success" : "backend_error",
      });
      const forwardedResponse = await forwardBackendResponse(response);
      await options.decorateResponse?.(request, forwardedResponse, response.ok);
      return forwardedResponse;
    } catch {
      await recordOperatorBackendProxyMetric({
        method: "POST",
        endpoint: backendEndpoint,
        status: 0,
        durationMs: 0,
        outcome: "network_error",
      });
      return NextResponse.json(
        { message: "Ein interner Fehler ist aufgetreten" },
        { status: 500 },
      );
    }
  };
}

/**
 * Creates an authenticated route handler for an arbitrary HTTP method against
 * a dynamic backend endpoint resolved from route params. Handles 401 → token
 * refresh → retry, and returns a TOKEN_EXPIRED envelope if refresh fails
 * (unlike createOperatorProxyPostHandler which forwards the backend 401
 * verbatim — these handlers are used where the client needs a reliable
 * sentinel to trigger a login redirect).
 *
 * Intended for methods without a request body (DELETE, parameterless POST).
 */
export function createOperatorProxyMethodHandler(
  method: string,
  endpointBuilder: (params: Record<string, unknown>) => string,
) {
  return withOperatorAuth(async (request, context): Promise<NextResponse> => {
    try {
      const session = await readOperatorSession();
      if (!session?.user?.token) {
        return NextResponse.json(
          { error: "Unauthorized", code: "TOKEN_EXPIRED" },
          { status: 401 },
        );
      }

      const [params, { getServerApiUrl }, { getClientForwardHeaders }] =
        await Promise.all([
          extractParams(request, context),
          import("~/lib/server-api-url"),
          import("~/lib/client-headers.server"),
        ]);
      const backendEndpoint = endpointBuilder(params);
      const url = `${getServerApiUrl()}${backendEndpoint}`;
      const forwardHeaders = getClientForwardHeaders(request);
      const startedAt = Date.now();

      const makeRequest = async (token: string) =>
        fetch(url, {
          method,
          headers: {
            Authorization: `Bearer ${token}`,
            "Content-Type": "application/json",
            ...forwardHeaders,
          },
        });

      let response = await makeRequest(session.user.token);

      if (response.status === 401) {
        const retried = await tryRetryWithRefreshedToken(
          session.user.token,
          async (newToken) => makeRequest(newToken),
        );
        if (retried) response = retried;
      }

      if (response.status === 401) {
        return NextResponse.json(
          { error: "Token expired", code: "TOKEN_EXPIRED" },
          { status: 401 },
        );
      }

      await recordOperatorBackendProxyMetric({
        method,
        endpoint: backendEndpoint,
        status: response.status,
        durationMs: Date.now() - startedAt,
        outcome: response.ok ? "success" : "backend_error",
      });
      return forwardBackendResponse(response);
    } catch (error) {
      return handleApiError(error);
    }
  });
}

/**
 * Operator proxy factories — envelope-wrapping pass-throughs built on the
 * operator base handlers (operator fetchers already unwrap `data`). These are
 * distinct from {@link createOperatorProxyPostHandler} /
 * {@link createOperatorProxyMethodHandler}, which forward the backend response
 * verbatim; use those when the client needs raw status codes/error messages.
 */
export const { proxyGet, proxyPost, proxyPut, proxyDelete } =
  makeProxyFactories({
    get: createOperatorGetHandler,
    post: createOperatorPostHandler,
    put: createOperatorPutHandler,
    del: createOperatorDeleteHandler,
    apiGet: operatorApiGet,
    apiPost: operatorApiPost,
    apiPut: operatorApiPut,
    apiDelete: operatorApiDelete,
    fetcherUnwrapsData: true,
  });

export { isStringParam } from "../route-wrapper-utils.server";
