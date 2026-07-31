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

export { isStringParam } from "./route-wrapper-utils.server";

/**
 * Checks if error is a 401 authentication error
 */
function is401Error(error: unknown): boolean {
  return error instanceof Error && error.message.includes("API error (401)");
}

/**
 * Creates a standard token expired response
 */
function createTokenExpiredResponse(): NextResponse<ApiErrorResponse> {
  return NextResponse.json(
    { error: "Token expired", code: "TOKEN_EXPIRED" },
    { status: 401 },
  );
}

/**
 * Attempts to retry a request with a refreshed token
 * Returns null if retry should not be attempted
 */
async function tryRetryWithRefreshedToken<T>(
  originalToken: string,
  retryFn: (token: string) => Promise<T>,
): Promise<T | null> {
  const updatedSession = await uncachedAuth();

  // Only retry if token was actually refreshed
  if (
    !updatedSession?.user?.token ||
    updatedSession.user.token === originalToken
  ) {
    return null;
  }

  return retryFn(updatedSession.user.token);
}

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
 * Executes handler with retry logic on 401 errors
 */
async function executeWithRetry<T>(
  token: string,
  executeHandler: (token: string) => Promise<T>,
  formatResponse: (data: T) => ApiNextResponse<T>,
): Promise<ApiNextResponse<T>> {
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

    return createTokenExpiredResponse();
  }
}

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
        return createUnauthorizedResponse();
      }

      const safeParams = await extractParams(request, context);
      const executeHandler = (token: string) =>
        handler(request, token, safeParams);

      return await executeWithRetry(
        session.user.token,
        executeHandler,
        (data) => formatResponse(data, request),
      );
    } catch (error) {
      return handleApiError(error);
    }
  });
}

/**
 * Base route handler for requests with body (POST, PUT)
 */
function createWithBodyHandler<T, B>(
  handler: WithBodyHandler<T, B>,
  formatResponse: ResponseFormatter<T>,
) {
  return withTenantAuth(async (request, context): RouteHandlerResponse<T> => {
    try {
      const session = await auth();
      if (!session?.user?.token) {
        return createUnauthorizedResponse();
      }

      const safeParams = await extractParams(request, context);
      const body = await parseRequestBody<B>(request);
      const executeHandler = (token: string) =>
        handler(request, body, token, safeParams);

      return await executeWithRetry(
        session.user.token,
        executeHandler,
        (data) => formatResponse(data, request),
      );
    } catch (error) {
      return handleApiError(error);
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
