// lib/api-helpers.server.ts
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { recordBackendProxyMetric } from "./backend-proxy-metrics";
import { canonicalForwardedFor } from "./client-headers.server";
import { sanitizeEndpoint } from "./log-sanitize";
import { createLogger } from "~/lib/logger";

// Logger instance for API helpers
const logger = createLogger({ component: "ApiHelpers" });

/**
 * Type for API response to ensure consistent structure
 */
export interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

/**
 * Type for error response
 */
export interface ApiErrorResponse {
  error: string;
  status?: number;
  code?: string;
  // Structured payload mirrored from the backend's ErrResponse.details.
  // Forwarded through the proxy so codes like reopen_status_conflict can
  // carry identifying fields (session_id, existing_status, …) to the UI.
  details?: Record<string, unknown>;
  // Top-level conflict list of the companion-plan 409 (student PUT). The
  // browser's CompanionPlanConflictError reads it to build the confirmation it
  // re-sends as confirmed_companion_extensions — dropping it here would make
  // "Ergänzen und speichern" loop on the same 409 forever.
  conflicts?: unknown[];
}

/**
 * HTTP methods supported by the API helpers
 */
type HttpMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

type JsonBody = Record<string, unknown> | unknown[] | string | number | boolean;

interface BackendErrorPayload {
  error?: string;
  message?: string;
  code?: string;
  details?: Record<string, unknown>;
  conflicts?: unknown[];
}

/**
 * Typed error thrown by serverFetchWithRetry / clientAxiosRequest for non-OK
 * backend responses. Carries the HTTP status and the raw body so callers can
 * branch on either without re-parsing `.message` strings. The `message`
 * intentionally keeps the legacy `"API error (<status>): <body>"` shape so
 * existing string-based parsers (handleApiError) and any caller catching it as
 * a plain Error continue to work unchanged.
 */
export class ApiResponseError extends Error {
  readonly status: number;
  readonly bodyText: string;
  // Memoized parse result — `null` means "not JSON". Lazy so callers that
  // only check status never pay the JSON.parse cost.
  private parsedBody: JsonBody | null = null;
  private parseAttempted = false;

  constructor(status: number, bodyText: string, options?: ErrorOptions) {
    super(`API error (${status}): ${bodyText}`, options);
    this.name = "ApiResponseError";
    this.status = status;
    this.bodyText = bodyText;
  }

  /**
   * Returns the parsed JSON body, or `null` if the body isn't valid JSON.
   * Use this instead of grepping `error.message` for backend error codes.
   */
  body<T = unknown>(): T | null {
    if (!this.parseAttempted) {
      this.parseAttempted = true;
      try {
        this.parsedBody = JSON.parse(this.bodyText) as JsonBody;
      } catch {
        this.parsedBody = null;
      }
    }
    return (this.parsedBody ?? null) as T | null;
  }
}

/**
 * Options for server-side fetch requests
 */
interface ServerFetchOptions {
  method: HttpMethod;
  body?: unknown;
  returnVoidOn204?: boolean;
}

async function getIncomingForwardHeaders(): Promise<Record<string, string>> {
  try {
    const { headers } = await import("next/headers");
    const incomingHeaders = await headers();
    const userAgent = incomingHeaders.get("user-agent");
    const forwardedFor = canonicalForwardedFor(incomingHeaders);

    return {
      ...(forwardedFor && {
        "X-Forwarded-For": forwardedFor,
      }),
      ...(userAgent && { "User-Agent": userAgent }),
    };
  } catch {
    return {};
  }
}

/**
 * Check authentication from inside a response-aware route wrapper.
 *
 * New route handlers should normally use the shared route factories instead;
 * those factories guarantee that a refreshed session cookie reaches the
 * browser. This helper remains for compatibility with existing callers and
 * must not be used by an unwrapped route handler.
 */
export async function checkAuth(): Promise<Response | null> {
  const { auth } = await import("../server/auth");
  const session = await auth();

  if (!session?.user?.token) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }

  return null;
}

/**
 * Server-side fetch against the backend API.
 * Route-level 401 retry with a refreshed session happens in route-wrapper.ts.
 */
async function serverFetchWithRetry<T>(
  endpoint: string,
  token: string,
  options: ServerFetchOptions,
): Promise<T> {
  const { getServerApiUrl } = await import("~/lib/server-api-url");
  const url = `${getServerApiUrl()}${endpoint}`;
  const forwardHeaders = await getIncomingForwardHeaders();
  const startedAt = Date.now();
  const requestBytes = estimateRequestBytes(options.body);
  const tokenContext = extractTokenContext(token);
  let responseStatus = 0;
  let responseBytes: number | null = null;
  let outcome: "success" | "backend_error" | "network_error" = "success";

  const executeRequest = async (bearer: string) =>
    fetch(url, {
      method: options.method,
      headers: {
        Authorization: `Bearer ${bearer}`,
        "Content-Type": "application/json",
        ...forwardHeaders,
      },
      body: options.body ? JSON.stringify(options.body) : undefined,
      cache: "no-store", // Prevent Next.js from caching API responses
    });

  try {
    const response = await executeRequest(token);
    responseStatus = response.status;
    responseBytes = parseResponseContentLength(response);

    if (!response.ok) {
      outcome = "backend_error";
      const errorText = await response.text();
      throw new ApiResponseError(response.status, errorText);
    }

    if (response.status === 204) {
      // DELETE should return void/undefined, others return empty object
      if (options.returnVoidOn204) {
        return undefined as T;
      }
      return {} as T;
    }

    return (await response.json()) as T;
  } catch (error) {
    if (outcome === "success") {
      outcome = "network_error";
    }
    throw error;
  } finally {
    const durationMs = Date.now() - startedAt;
    const backendEndpoint = sanitizeEndpoint(endpoint);

    logger.info("frontend backend proxy request completed", {
      method: options.method,
      backend_endpoint: backendEndpoint,
      status: responseStatus,
      duration_ms: durationMs,
      request_bytes: requestBytes,
      response_bytes: responseBytes,
      tenant_id: tokenContext.tenantId,
      org_id: tokenContext.orgId,
      scope: tokenContext.scope,
      outcome,
    });
    await recordTenantBackendProxyMetric({
      method: options.method,
      backendEndpoint,
      status: responseStatus,
      durationMs,
      outcome,
      scope: tokenContext.scope,
    });
  }
}

async function recordTenantBackendProxyMetric(args: {
  method: string;
  backendEndpoint: string;
  status: number;
  durationMs: number;
  outcome: string;
  scope?: string;
}): Promise<void> {
  try {
    recordBackendProxyMetric(args);
  } catch {
    // Metrics must never alter request behavior.
  }
}

function estimateRequestBytes(body: unknown): number {
  if (body === undefined || body === null) return 0;
  try {
    return new TextEncoder().encode(JSON.stringify(body)).length;
  } catch {
    return 0;
  }
}

function parseContentLength(value: string | null): number | null {
  if (!value) return null;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
}

function parseResponseContentLength(response: Response): number | null {
  return parseContentLength(response.headers?.get?.("content-length") ?? null);
}

function extractTokenContext(token: string): {
  tenantId?: number;
  orgId?: number;
  scope?: string;
} {
  try {
    const payload = token.split(".")[1];
    if (!payload) return {};
    const decoded = JSON.parse(Buffer.from(payload, "base64url").toString());
    return {
      tenantId:
        typeof decoded.tenant_id === "number" ? decoded.tenant_id : undefined,
      orgId: typeof decoded.org_id === "number" ? decoded.org_id : undefined,
      scope: typeof decoded.scope === "string" ? decoded.scope : undefined,
    };
  } catch {
    return {};
  }
}

/**
 * Make a GET request to the API
 * @param endpoint API endpoint to request
 * @param token Authentication token
 * @returns Promise with the response data
 */
export async function apiGet<T>(endpoint: string, token: string): Promise<T> {
  return serverFetchWithRetry<T>(endpoint, token, { method: "GET" });
}

/**
 * Make a POST request to the API
 * @param endpoint API endpoint to request
 * @param token Authentication token
 * @param body Request body
 * @returns Promise with the response data
 */
export async function apiPost<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return serverFetchWithRetry<T>(endpoint, token, { method: "POST", body });
}

/**
 * Make a PUT request to the API
 * @param endpoint API endpoint to request
 * @param token Authentication token
 * @param body Request body
 * @returns Promise with the response data
 */
export async function apiPut<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return serverFetchWithRetry<T>(endpoint, token, { method: "PUT", body });
}

/**
 * Make a PATCH request to the API
 * @param endpoint API endpoint to request
 * @param token Authentication token
 * @param body Request body
 * @returns Promise with the response data
 */
export async function apiPatch<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return serverFetchWithRetry<T>(endpoint, token, { method: "PATCH", body });
}

/**
 * Make a DELETE request to the API
 * @param endpoint API endpoint to request
 * @param token Authentication token
 * @returns Promise with the response data, or void for 204 No Content responses
 */
export async function apiDelete<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T | void> {
  return serverFetchWithRetry<T>(endpoint, token, {
    method: "DELETE",
    body,
    returnVoidOn204: true,
  });
}

/**
 * Handler for API errors
 * @param error Error object
 * @returns Response with error message and status
 */
export function handleApiError(error: unknown): NextResponse<ApiErrorResponse> {
  const status = getApiErrorStatus(error);
  if (error instanceof Error && status !== null) {
    logApiRouteError(status, error.message);
    return NextResponse.json(buildApiErrorResponse(error.message), { status });
  }

  // Unknown errors are logged as errors and return 500
  logger.error("api route error without status", {
    error: error instanceof Error ? error.message : String(error),
  });
  const errorMessage =
    error instanceof Error ? error.message : "Internal Server Error";
  return NextResponse.json({ error: errorMessage }, { status: 500 });
}

function getApiErrorStatus(error: unknown): number | null {
  if (!(error instanceof Error)) {
    return null;
  }

  // Match both "API error: 403" and "API error (403):" formats (exactly 3 digits)
  const regex = /API error[:\s(]+(\d{3})/;
  const match = regex.exec(error.message);
  return match?.[1] ? Number.parseInt(match[1], 10) : null;
}

function logApiRouteError(status: number, errorMessage: string): void {
  // Free-text fields like sick-note content must not land in server logs even
  // when a 4xx body still carries them. Redact before any level logs the body.
  const safeErrorMessage = redactSensitiveApiErrorFields(errorMessage);

  // Only log server errors (5xx) to avoid Next.js error overlay for expected 4xx
  if (status >= 500) {
    logger.error("api route error", {
      status,
      error: safeErrorMessage,
    });
    return;
  }

  if (status === 429) {
    logger.warn("api route rate limited", {
      status,
      error: safeErrorMessage,
      rate_limited: true,
    });
    return;
  }

  logger.warn("api route error", {
    status,
    error: safeErrorMessage,
  });
}

/** JSON keys whose free-text values must not appear in server logs. */
const SENSITIVE_API_ERROR_KEYS = new Set(["note"]);

function redactSensitiveApiErrorFields(errorMessage: string): string {
  const jsonStart = errorMessage.indexOf("{");
  if (jsonStart === -1) {
    return errorMessage;
  }

  try {
    const payload = JSON.parse(errorMessage.substring(jsonStart)) as unknown;
    return (
      errorMessage.substring(0, jsonStart) +
      JSON.stringify(redactSensitiveApiErrorValue(payload))
    );
  } catch {
    return errorMessage;
  }
}

function redactSensitiveApiErrorValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map((entry) => redactSensitiveApiErrorValue(entry));
  }
  if (value === null || typeof value !== "object") {
    return value;
  }

  const redacted: Record<string, unknown> = {};
  for (const [key, entry] of Object.entries(value as Record<string, unknown>)) {
    if (SENSITIVE_API_ERROR_KEYS.has(key) && entry != null && entry !== "") {
      redacted[key] = "[REDACTED]";
      continue;
    }
    redacted[key] = redactSensitiveApiErrorValue(entry);
  }
  return redacted;
}

function buildApiErrorResponse(errorMessage: string): ApiErrorResponse {
  // Try to extract structured fields from embedded backend JSON response.
  // Mirror the wire shape: error, code, AND details — the proxy must not
  // silently drop details, or codes like reopen_status_conflict that carry
  // identifying fields lose them between backend and browser (Issue #1368).
  const response: ApiErrorResponse = { error: errorMessage };
  const parsed = parseBackendErrorPayload(errorMessage);

  if (parsed?.error ?? parsed?.message) {
    response.error = (parsed.error ?? parsed.message) as string;
  }
  if (parsed?.code) {
    response.code = parsed.code;
  }
  if (parsed?.details) {
    response.details = parsed.details;
  }
  if (Array.isArray(parsed?.conflicts)) {
    response.conflicts = parsed.conflicts;
  }

  return response;
}

function parseBackendErrorPayload(
  errorMessage: string,
): BackendErrorPayload | null {
  const jsonStart = errorMessage.indexOf("{");
  if (jsonStart === -1) {
    return null;
  }

  try {
    return JSON.parse(errorMessage.substring(jsonStart)) as BackendErrorPayload;
  } catch {
    return null;
  }
}

/**
 * Extract URL parameters from the request
 * @param request NextRequest object
 * @param params Parameter names to extract
 * @returns Object with extracted parameters
 */
export function extractParams(
  request: NextRequest,
  params: Record<string, unknown>,
): Record<string, string> {
  const urlParams: Record<string, string> = {};

  // Extract from URL params object
  Object.keys(params).forEach((key) => {
    if (params[key] && typeof params[key] === "string") {
      urlParams[key] = params[key];
    }
  });

  // Extract from query params
  const searchParams = request.nextUrl.searchParams;
  searchParams.forEach((value, key) => {
    urlParams[key] = value;
  });

  return urlParams;
}
