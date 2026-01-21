/**
 * Route Handler Wrapper for Next.js API Routes
 *
 * This module provides wrapper functions for API route handlers that:
 * 1. Forward cookies to the Go backend (BetterAuth session validation)
 * 2. Handle authentication and authorization
 * 3. Provide consistent error handling
 * 4. Support retry logic for transient failures
 *
 * BetterAuth Migration Notes:
 * - No more JWT tokens - session is validated via cookies
 * - Go backend validates session with BetterAuth service
 * - Cookies are forwarded automatically from request headers
 */

import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { cookies } from "next/headers";
import type { ApiErrorResponse, ApiResponse } from "./api-helpers";
import { handleApiError } from "./api-helpers";
import { env } from "~/env";

/**
 * Helper to build query string from request search params
 */
function buildQueryString(request: NextRequest): string {
  const queryParams = new URLSearchParams();
  request.nextUrl.searchParams.forEach((value, key) => {
    queryParams.append(key, value);
  });
  const queryString = queryParams.toString();
  return queryString ? `?${queryString}` : "";
}

/**
 * Type guard to check if parameter exists and is a string
 */
export function isStringParam(param: unknown): param is string {
  return typeof param === "string";
}

/**
 * Extracts parameters from context and URL
 */
async function extractParams(
  request: NextRequest,
  context: { params: Promise<Record<string, string | string[] | undefined>> },
): Promise<Record<string, unknown>> {
  const safeParams: Record<string, unknown> = {};

  // Get params from context
  const contextParams = await context.params;
  if (contextParams) {
    Object.entries(contextParams).forEach(([key, value]) => {
      if (value !== undefined) {
        safeParams[key] = value;
      }
    });
  }

  // Extract parameters from URL path
  const url = new URL(request.url);
  const pathParts = url.pathname.split("/");

  // Try to extract ID from URL path parts if not already set
  if (!safeParams.id) {
    const potentialIds = pathParts.filter((part) => /^\d+$/.test(part));
    if (potentialIds.length > 0) {
      safeParams.id = potentialIds.at(-1);
    }
  }

  // Extract search params
  url.searchParams.forEach((value, key) => {
    safeParams[key] = value;
  });

  return safeParams;
}

/**
 * Wraps data in ApiResponse format if not already wrapped
 */
function wrapInApiResponse<T>(data: T): ApiResponse<T> {
  if (typeof data === "object" && data !== null && "success" in data) {
    return data as unknown as ApiResponse<T>;
  }
  return { success: true, message: "Success", data };
}

/**
 * Creates a standard unauthorized response
 */
function createUnauthorizedResponse(): NextResponse<ApiErrorResponse> {
  return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
}

/**
 * Get cookies as a header string for forwarding to backend
 * BetterAuth uses cookies for session management
 */
async function getCookieHeader(): Promise<string> {
  const cookieStore = await cookies();
  return cookieStore.toString();
}

/**
 * Check if the user has an active session by checking for BetterAuth session cookie
 */
async function hasActiveSession(): Promise<boolean> {
  const cookieStore = await cookies();
  // BetterAuth session cookie name
  const sessionCookie = cookieStore.get("better-auth.session_token");
  return !!sessionCookie?.value;
}

/**
 * Route context type for Next.js 15+
 */
type RouteContext = {
  params: Promise<Record<string, string | string[] | undefined>>;
};

/**
 * Standard API response wrapped in NextResponse
 */
type ApiNextResponse<T> = NextResponse<ApiResponse<T> | ApiErrorResponse | T>;

/**
 * Standard route handler return type (async)
 */
type RouteHandlerResponse<T> = Promise<ApiNextResponse<T>>;

/**
 * Handler type without body (GET, DELETE)
 * Note: Now receives cookieHeader instead of token
 */
type NoBodyHandler<T> = (
  request: NextRequest,
  cookieHeader: string,
  params: Record<string, unknown>,
) => Promise<T>;

/**
 * Handler type with body (POST, PUT)
 * Note: Now receives cookieHeader instead of token
 */
type WithBodyHandler<T, B> = (
  request: NextRequest,
  body: B,
  cookieHeader: string,
  params: Record<string, unknown>,
) => Promise<T>;

/**
 * Response formatter type
 */
type ResponseFormatter<T> = (
  data: T,
  request: NextRequest,
) => ApiNextResponse<T>;

/**
 * Handles GET response formatting, including special case for rooms endpoint
 */
function formatGetResponse<T>(
  data: T,
  pathname: string,
): NextResponse<ApiResponse<T> | T> {
  // For the rooms endpoint, pass raw data directly
  if (pathname === "/api/rooms") {
    return NextResponse.json(data);
  }
  return NextResponse.json(wrapInApiResponse(data));
}

/**
 * Formats DELETE response, returning 204 for null/undefined data
 */
function formatDeleteResponse<T>(data: T): ApiNextResponse<T> {
  if (data === null || data === undefined) {
    // Return 204 No Content for successful deletions without body
    return new NextResponse(null, { status: 204 });
  }
  return NextResponse.json(wrapInApiResponse(data));
}

/**
 * Parses JSON body from request, returns empty object on failure
 */
async function parseRequestBody<B>(request: NextRequest): Promise<B> {
  try {
    const text = await request.text();
    return text ? (JSON.parse(text) as B) : ({} as B);
  } catch {
    return {} as B;
  }
}

/**
 * Base route handler for requests without body (GET, DELETE)
 * Forwards cookies to backend for BetterAuth session validation
 */
function createNoBodyHandler<T>(
  handler: NoBodyHandler<T>,
  formatResponse: ResponseFormatter<T>,
) {
  return async (
    request: NextRequest,
    context: RouteContext,
  ): RouteHandlerResponse<T> => {
    try {
      // Check if user has an active session
      if (!(await hasActiveSession())) {
        return createUnauthorizedResponse();
      }

      const cookieHeader = await getCookieHeader();
      const safeParams = await extractParams(request, context);

      const data = await handler(request, cookieHeader, safeParams);
      return formatResponse(data, request);
    } catch (error) {
      return handleApiError(error);
    }
  };
}

/**
 * Base route handler for requests with body (POST, PUT)
 * Forwards cookies to backend for BetterAuth session validation
 */
function createWithBodyHandler<T, B>(
  handler: WithBodyHandler<T, B>,
  formatResponse: ResponseFormatter<T>,
) {
  return async (
    request: NextRequest,
    context: RouteContext,
  ): RouteHandlerResponse<T> => {
    try {
      // Check if user has an active session
      if (!(await hasActiveSession())) {
        return createUnauthorizedResponse();
      }

      const cookieHeader = await getCookieHeader();
      const safeParams = await extractParams(request, context);
      const body = await parseRequestBody<B>(request);

      const data = await handler(request, body, cookieHeader, safeParams);
      return formatResponse(data, request);
    } catch (error) {
      return handleApiError(error);
    }
  };
}

// ============================================================================
// API Request Helpers - Forward cookies to Go backend
// ============================================================================

/**
 * Make a GET request to the Go backend, forwarding cookies for auth
 */
export async function apiGetWithCookies<T>(
  endpoint: string,
  cookieHeader: string,
): Promise<T> {
  const url = `${env.NEXT_PUBLIC_API_URL}${endpoint}`;

  const response = await fetch(url, {
    method: "GET",
    headers: {
      Cookie: cookieHeader,
      "Content-Type": "application/json",
    },
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`API error (${response.status}): ${errorText}`);
  }

  if (response.status === 204) {
    return {} as T;
  }

  return (await response.json()) as T;
}

/**
 * Make a POST request to the Go backend, forwarding cookies for auth
 */
export async function apiPostWithCookies<T, B = unknown>(
  endpoint: string,
  cookieHeader: string,
  body?: B,
): Promise<T> {
  const url = `${env.NEXT_PUBLIC_API_URL}${endpoint}`;

  const response = await fetch(url, {
    method: "POST",
    headers: {
      Cookie: cookieHeader,
      "Content-Type": "application/json",
    },
    body: body ? JSON.stringify(body) : undefined,
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`API error (${response.status}): ${errorText}`);
  }

  if (response.status === 204) {
    return {} as T;
  }

  return (await response.json()) as T;
}

/**
 * Make a PUT request to the Go backend, forwarding cookies for auth
 */
export async function apiPutWithCookies<T, B = unknown>(
  endpoint: string,
  cookieHeader: string,
  body?: B,
): Promise<T> {
  const url = `${env.NEXT_PUBLIC_API_URL}${endpoint}`;

  const response = await fetch(url, {
    method: "PUT",
    headers: {
      Cookie: cookieHeader,
      "Content-Type": "application/json",
    },
    body: body ? JSON.stringify(body) : undefined,
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`API error (${response.status}): ${errorText}`);
  }

  if (response.status === 204) {
    return {} as T;
  }

  return (await response.json()) as T;
}

/**
 * Make a DELETE request to the Go backend, forwarding cookies for auth
 */
export async function apiDeleteWithCookies<T>(
  endpoint: string,
  cookieHeader: string,
): Promise<T | void> {
  const url = `${env.NEXT_PUBLIC_API_URL}${endpoint}`;

  const response = await fetch(url, {
    method: "DELETE",
    headers: {
      Cookie: cookieHeader,
      "Content-Type": "application/json",
    },
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`API error (${response.status}): ${errorText}`);
  }

  if (response.status === 204) {
    return undefined;
  }

  return (await response.json()) as T;
}

// ============================================================================
// Proxy Handler Factories
// ============================================================================

/**
 * Creates a simple proxy GET handler that forwards query params to backend
 * Use this for routes that just pass through to the backend without transformation
 * @param backendEndpoint The backend API endpoint (e.g., "/api/active/groups")
 */
export function createProxyGetHandler<T>(backendEndpoint: string) {
  return createGetHandler<T>(
    async (request: NextRequest, cookieHeader: string) => {
      const endpoint = `${backendEndpoint}${buildQueryString(request)}`;
      return await apiGetWithCookies(endpoint, cookieHeader);
    },
  );
}

/**
 * Creates a proxy GET handler for routes with [id] parameter
 * @param backendEndpoint The backend API endpoint (e.g., "/api/active/groups")
 */
export function createProxyGetByIdHandler<T>(backendEndpoint: string) {
  return createGetHandler<T>(
    async (_request: NextRequest, cookieHeader: string, params) => {
      if (!isStringParam(params.id)) {
        throw new Error("Invalid id parameter");
      }
      return await apiGetWithCookies(
        `${backendEndpoint}/${params.id}`,
        cookieHeader,
      );
    },
  );
}

/**
 * Creates a proxy PUT handler for routes with [id] parameter
 * @param backendEndpoint The backend API endpoint (e.g., "/api/active/groups")
 */
export function createProxyPutHandler<T, B = unknown>(backendEndpoint: string) {
  return createPutHandler<T, B>(
    async (_request: NextRequest, body: B, cookieHeader: string, params) => {
      if (!isStringParam(params.id)) {
        throw new Error("Invalid id parameter");
      }
      return await apiPutWithCookies(
        `${backendEndpoint}/${params.id}`,
        cookieHeader,
        body,
      );
    },
  );
}

/**
 * Creates a proxy DELETE handler for routes with [id] parameter
 * @param backendEndpoint The backend API endpoint (e.g., "/api/active/groups")
 */
export function createProxyDeleteHandler(backendEndpoint: string) {
  return createDeleteHandler(
    async (_request: NextRequest, cookieHeader: string, params) => {
      if (!isStringParam(params.id)) {
        throw new Error("Invalid id parameter");
      }
      await apiDeleteWithCookies(
        `${backendEndpoint}/${params.id}`,
        cookieHeader,
      );
      return null;
    },
  );
}

// ============================================================================
// Handler Factories
// ============================================================================

// Shared formatter for POST/PUT handlers that return JSON with API response wrapper
const formatBodyHandlerResponse = <T>(data: T) =>
  NextResponse.json(wrapInApiResponse(data));

/**
 * Wrapper function for handling GET API routes
 * @param handler Function that handles the API request
 * @returns Response from the handler or error response
 */
export function createGetHandler<T>(handler: NoBodyHandler<T>) {
  return createNoBodyHandler(handler, (data, request) =>
    formatGetResponse(data, request.nextUrl.pathname),
  );
}

/**
 * Wrapper function for handling POST API routes
 * @param handler Function that handles the API request
 * @returns Response from the handler or error response
 */
export function createPostHandler<T, B = unknown>(
  handler: WithBodyHandler<T, B>,
) {
  return createWithBodyHandler(handler, formatBodyHandlerResponse);
}

/**
 * Wrapper function for handling PUT API routes
 * Uses the same response format as POST handlers (both modify resources)
 * @param handler Function that handles the API request
 * @returns Response from the handler or error response
 */
export function createPutHandler<T, B = unknown>(
  handler: WithBodyHandler<T, B>,
) {
  return createWithBodyHandler(handler, formatBodyHandlerResponse);
}

/**
 * Wrapper function for handling DELETE API routes
 * @param handler Function that handles the API request
 * @returns Response from the handler or error response
 */
export function createDeleteHandler<T>(handler: NoBodyHandler<T>) {
  return createNoBodyHandler(handler, (data) => formatDeleteResponse(data));
}

// ============================================================================
// Legacy Token-based API helpers (for compatibility during migration)
// These are wrappers that accept a "token" parameter but actually use cookies
// ============================================================================

import { apiGet, apiPost, apiPut, apiDelete } from "./api-helpers";

export { apiGet, apiPost, apiPut, apiDelete };
