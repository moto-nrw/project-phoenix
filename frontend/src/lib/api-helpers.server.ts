// lib/api-helpers.server.ts
import type { NextRequest } from "next/server";
import { recordBackendProxyMetric } from "./backend-proxy-metrics";
import { analyticsSessionHeaders } from "./analytics-session-header.server";
import {
  canonicalForwardedFor,
  requestIdHeaders,
} from "./client-headers.server";
import { sanitizeEndpoint } from "./log-sanitize";
import { createLogger } from "~/lib/logger";
import type { ProblemBody } from "./api-error";
import { ApiResponseError } from "./api-response-error.server";

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
 * The error body the frontend's API routes answer with: the backend's
 * RFC 9457 envelope (ADR 0006), forwarded or built by `problemResponse`.
 */
export type ApiErrorResponse = ProblemBody;

export {
  ApiResponseError,
  backendResponseError,
  bffError,
  handleApiError,
  problemResponse,
} from "./api-response-error.server";

/**
 * HTTP methods supported by the API helpers
 */
type HttpMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

/**
 * Options for server-side fetch requests
 */
interface ServerFetchOptions {
  method: HttpMethod;
  body?: unknown;
  idempotencyKey?: string;
  returnVoidOn204?: boolean;
  signal?: AbortSignal;
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
      ...analyticsSessionHeaders(incomingHeaders),
      ...requestIdHeaders(incomingHeaders),
    };
  } catch {
    return {};
  }
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
        ...(options.idempotencyKey && {
          "Idempotency-Key": options.idempotencyKey,
        }),
      },
      body: options.body ? JSON.stringify(options.body) : undefined,
      cache: "no-store", // Prevent Next.js from caching API responses
      ...(options.signal && { signal: options.signal }),
    });

  try {
    const response = await executeRequest(token);
    responseStatus = response.status;
    responseBytes = parseResponseContentLength(response);

    if (!response.ok) {
      outcome = "backend_error";
      const errorText = await response.text();
      throw new ApiResponseError(response.status, errorText, {
        retryAfter: response.headers.get("Retry-After"),
        contentType: response.headers.get("Content-Type"),
      });
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
export async function apiGet<T>(
  endpoint: string,
  token: string,
  options?: { signal?: AbortSignal },
): Promise<T> {
  return serverFetchWithRetry<T>(endpoint, token, {
    method: "GET",
    signal: options?.signal,
  });
}

/**
 * Make a POST request to the API
 * @param endpoint API endpoint to request
 * @param token Authentication token
 * @param body Request body
 * @param idempotencyKey Optional key that makes a retried create request safe
 * @returns Promise with the response data
 */
export async function apiPost<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
  idempotencyKey?: string,
): Promise<T> {
  return serverFetchWithRetry<T>(endpoint, token, {
    method: "POST",
    body,
    idempotencyKey,
  });
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
