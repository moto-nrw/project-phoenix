import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth, uncachedAuth } from "../server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import type { ApiErrorResponse, ApiResponse } from "./api-helpers.server";
import { handleApiError } from "./api-helpers.server";
import {
  extractParams,
  parseRequestBody,
  wrapInApiResponse,
  createUnauthorizedResponse,
} from "./route-wrapper-utils.server";
import { executeWithTokenRefresh } from "./token-refresh-retry.server";

export { isStringParam } from "./route-wrapper-utils.server";

// Read on a retry only: route tests mock ~/server/auth without it.
const readFreshTenantSession = () => uncachedAuth();

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
 * Standard API response wrapped in NextResponse
 */
type ApiNextResponse<T> = NextResponse<ApiResponse<T> | ApiErrorResponse | T>;

/**
 * Standard route handler return type (async)
 */
type RouteHandlerResponse<T> = Promise<ApiNextResponse<T>>;

/**
 * Handler type without body (GET, DELETE)
 */
type NoBodyHandler<T> = (
  request: NextRequest,
  token: string,
  params: Record<string, unknown>,
) => Promise<T>;

/**
 * Handler type with body (POST, PUT)
 */
type WithBodyHandler<T, B> = (
  request: NextRequest,
  body: B,
  token: string,
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
 * Base route handler for requests without body (GET, DELETE)
 */
function createNoBodyHandler<T>(
  handler: NoBodyHandler<T>,
  formatResponse: ResponseFormatter<T>,
) {
  return withTenantAuth(async (request, context): RouteHandlerResponse<T> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return createUnauthorizedResponse(request);
      }

      const safeParams = await extractParams(request, context);
      const executeHandler = (token: string) =>
        handler(request, token, safeParams);

      return await executeWithTokenRefresh(
        request,
        session.user.token,
        executeHandler,
        (data) => formatResponse(data, request),
        readFreshTenantSession,
      );
    } catch (error) {
      return handleApiError(error, request);
    }
  });
}

/**
 * Options for body-carrying route handlers.
 */
interface BodyHandlerOptions {
  /**
   * Enforce this byte limit while READING the request body — an oversized
   * payload is rejected with 413 before it is buffered or parsed, mirroring
   * a backend-side MaxBytesReader bound.
   */
  maxBodyBytes?: number;
}

/**
 * Base route handler for requests with body (POST, PUT)
 */
function createWithBodyHandler<T, B>(
  handler: WithBodyHandler<T, B>,
  formatResponse: ResponseFormatter<T>,
  options?: BodyHandlerOptions,
) {
  return withTenantAuth(async (request, context): RouteHandlerResponse<T> => {
    try {
      const session = await auth();
      if (!session?.user?.token) {
        return createUnauthorizedResponse(request);
      }

      const safeParams = await extractParams(request, context);
      const body = await parseRequestBody<B>(request, options?.maxBodyBytes);
      const executeHandler = (token: string) =>
        handler(request, body, token, safeParams);

      return await executeWithTokenRefresh(
        request,
        session.user.token,
        executeHandler,
        (data) => formatResponse(data, request),
        readFreshTenantSession,
      );
    } catch (error) {
      return handleApiError(error, request);
    }
  });
}

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

// Shared formatter for POST/PUT handlers that return JSON with API response wrapper
const formatBodyHandlerResponse = <T>(data: T) =>
  NextResponse.json(wrapInApiResponse(data));

/**
 * Wrapper function for handling POST API routes
 * @param handler Function that handles the API request
 * @returns Response from the handler or error response
 */
export function createPostHandler<T, B = unknown>(
  handler: WithBodyHandler<T, B>,
  options?: BodyHandlerOptions,
) {
  return createWithBodyHandler(handler, formatBodyHandlerResponse, options);
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
 * Wrapper function for handling PATCH API routes
 * Uses the same response format as POST/PUT handlers.
 * @param handler Function that handles the API request
 * @returns Response from the handler or error response
 */
export function createPatchHandler<T, B = unknown>(
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

/**
 * DELETE variant for destructive workflows that require an explicit JSON
 * confirmation payload. Empty legacy requests still parse as an empty object,
 * letting the backend reject them with the domain-specific conflict response.
 */
export function createDeleteWithBodyHandler<T, B = unknown>(
  handler: WithBodyHandler<T, B>,
) {
  return createWithBodyHandler(handler, (data) => formatDeleteResponse(data));
}
