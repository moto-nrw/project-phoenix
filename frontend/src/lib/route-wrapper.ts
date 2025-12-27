import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "../server/auth";
import type { ApiErrorResponse, ApiResponse } from "./api-helpers";
import { apiDelete, apiGet, apiPut, handleApiError } from "./api-helpers";

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
      safeParams.id = potentialIds[potentialIds.length - 1];
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
  const updatedSession = await auth();

  // Only retry if token was actually refreshed
  if (
    !updatedSession?.user?.token ||
    updatedSession.user.token === originalToken
  ) {
    return null;
  }

  console.log("Token was refreshed, retrying request with new token");
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
 * Creates a simple proxy GET handler that forwards query params to backend
 * Use this for routes that just pass through to the backend without transformation
 * @param backendEndpoint The backend API endpoint (e.g., "/api/active/groups")
 */
export function createProxyGetHandler<T>(backendEndpoint: string) {
  return createGetHandler<T>(async (request: NextRequest, token: string) => {
    const endpoint = `${backendEndpoint}${buildQueryString(request)}`;
    return await apiGet(endpoint, token);
  });
}

/**
 * Creates a proxy GET handler for routes with [id] parameter
 * @param backendEndpoint The backend API endpoint (e.g., "/api/active/groups")
 */
export function createProxyGetByIdHandler<T>(backendEndpoint: string) {
  return createGetHandler<T>(
    async (_request: NextRequest, token: string, params) => {
      if (!isStringParam(params.id)) {
        throw new Error("Invalid id parameter");
      }
      return await apiGet(`${backendEndpoint}/${params.id}`, token);
    },
  );
}

/**
 * Creates a proxy PUT handler for routes with [id] parameter
 * @param backendEndpoint The backend API endpoint (e.g., "/api/active/groups")
 */
export function createProxyPutHandler<T, B = unknown>(backendEndpoint: string) {
  return createPutHandler<T, B>(
    async (_request: NextRequest, body: B, token: string, params) => {
      if (!isStringParam(params.id)) {
        throw new Error("Invalid id parameter");
      }
      return await apiPut(`${backendEndpoint}/${params.id}`, token, body);
    },
  );
}

/**
 * Creates a proxy DELETE handler for routes with [id] parameter
 * @param backendEndpoint The backend API endpoint (e.g., "/api/active/groups")
 */
export function createProxyDeleteHandler(backendEndpoint: string) {
  return createDeleteHandler(
    async (_request: NextRequest, token: string, params) => {
      if (!isStringParam(params.id)) {
        throw new Error("Invalid id parameter");
      }
      await apiDelete(`${backendEndpoint}/${params.id}`, token);
      return null;
    },
  );
}

/**
 * Wrapper function for handling GET API routes
 * @param handler Function that handles the API request
 * @returns Response from the handler or error response
 */
export function createGetHandler<T>(
  handler: (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => Promise<T>,
) {
  return async (
    request: NextRequest,
    context: { params: Promise<Record<string, string | string[] | undefined>> },
  ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse | T>> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
      }

      const safeParams = await extractParams(request, context);
      const pathname = request.nextUrl.pathname;

      try {
        const data = await handler(request, session.user.token, safeParams);
        return formatGetResponse(data, pathname);
      } catch (handlerError) {
        if (!is401Error(handlerError)) {
          throw handlerError;
        }

        // Try retry with refreshed token
        try {
          const retryData = await tryRetryWithRefreshedToken(
            session.user.token,
            (token) => handler(request, token, safeParams),
          );

          if (retryData !== null) {
            return formatGetResponse(retryData, pathname);
          }
        } catch {
          // Retry failed, fall through to token expired
        }

        return createTokenExpiredResponse();
      }
    } catch (error) {
      return handleApiError(error);
    }
  };
}

/**
 * Wrapper function for handling POST API routes
 * @param handler Function that handles the API request
 * @returns Response from the handler or error response
 */
export function createPostHandler<T, B = unknown>(
  handler: (
    request: NextRequest,
    body: B,
    token: string,
    params: Record<string, unknown>,
  ) => Promise<T>,
) {
  return async (
    request: NextRequest,
    context: { params: Promise<Record<string, string | string[] | undefined>> },
  ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse | T>> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
      }

      const safeParams = await extractParams(request, context);
      const body = await parseRequestBody<B>(request);

      try {
        const data = await handler(
          request,
          body,
          session.user.token,
          safeParams,
        );
        return NextResponse.json(wrapInApiResponse(data));
      } catch (handlerError) {
        if (!is401Error(handlerError)) {
          throw handlerError;
        }

        // Try retry with refreshed token
        try {
          const retryData = await tryRetryWithRefreshedToken(
            session.user.token,
            (token) => handler(request, body, token, safeParams),
          );

          if (retryData !== null) {
            return NextResponse.json(wrapInApiResponse(retryData));
          }
        } catch {
          // Retry failed, fall through to token expired
        }

        return createTokenExpiredResponse();
      }
    } catch (error) {
      return handleApiError(error);
    }
  };
}

/**
 * Wrapper function for handling PUT API routes
 * @param handler Function that handles the API request
 * @returns Response from the handler or error response
 */
export function createPutHandler<T, B = unknown>(
  handler: (
    request: NextRequest,
    body: B,
    token: string,
    params: Record<string, unknown>,
  ) => Promise<T>,
) {
  return async (
    request: NextRequest,
    context: { params: Promise<Record<string, string | string[] | undefined>> },
  ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse | T>> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
      }

      const safeParams = await extractParams(request, context);
      const body = await parseRequestBody<B>(request);

      try {
        const data = await handler(
          request,
          body,
          session.user.token,
          safeParams,
        );
        return NextResponse.json(wrapInApiResponse(data));
      } catch (handlerError) {
        if (!is401Error(handlerError)) {
          throw handlerError;
        }

        // Try retry with refreshed token
        try {
          const retryData = await tryRetryWithRefreshedToken(
            session.user.token,
            (token) => handler(request, body, token, safeParams),
          );

          if (retryData !== null) {
            return NextResponse.json(wrapInApiResponse(retryData));
          }
        } catch {
          // Retry failed, fall through to token expired
        }

        return createTokenExpiredResponse();
      }
    } catch (error) {
      return handleApiError(error);
    }
  };
}

/**
 * Wrapper function for handling DELETE API routes
 * @param handler Function that handles the API request
 * @returns Response from the handler or error response
 */
export function createDeleteHandler<T>(
  handler: (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => Promise<T>,
) {
  return async (
    request: NextRequest,
    context: { params: Promise<Record<string, string | string[] | undefined>> },
  ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse | T>> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
      }

      // Extract parameters from both context and URL
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
          // Use the last numeric part as ID
          safeParams.id = potentialIds[potentialIds.length - 1];
        }
      }

      // Extract search params
      url.searchParams.forEach((value, key) => {
        safeParams[key] = value;
      });

      try {
        const data = await handler(request, session.user.token, safeParams);

        // For delete operations with no content, return 204 status
        if (data === null || data === undefined) {
          return new NextResponse(null, { status: 204 });
        }

        // Wrap the response in ApiResponse format if it's not already
        const response: ApiResponse<T> =
          typeof data === "object" && data !== null && "success" in data
            ? (data as unknown as ApiResponse<T>)
            : { success: true, message: "Success", data };

        return NextResponse.json(response);
      } catch (handlerError) {
        // Check if it's a 401 error from the backend
        if (
          handlerError instanceof Error &&
          handlerError.message.includes("API error (401)")
        ) {
          // Try to get updated session in case it was refreshed
          const updatedSession = await auth();

          // If we have an updated token, retry once
          if (
            updatedSession?.user?.token &&
            updatedSession.user.token !== session.user.token
          ) {
            console.log(
              "Token was refreshed, retrying DELETE request with new token",
            );
            try {
              const retryData = await handler(
                request,
                updatedSession.user.token,
                safeParams,
              );

              // For delete operations with no content, return 204 status
              if (retryData === null || retryData === undefined) {
                return new NextResponse(null, { status: 204 });
              }

              // Wrap the response in ApiResponse format if it's not already
              const retryResponse: ApiResponse<T> =
                typeof retryData === "object" &&
                retryData !== null &&
                "success" in retryData
                  ? (retryData as unknown as ApiResponse<T>)
                  : { success: true, message: "Success", data: retryData };

              return NextResponse.json(retryResponse);
            } catch {
              // If retry also fails, return 401
              return NextResponse.json(
                { error: "Token expired", code: "TOKEN_EXPIRED" },
                { status: 401 },
              );
            }
          }

          // Return 401 to client so it can handle token refresh
          return NextResponse.json(
            { error: "Token expired", code: "TOKEN_EXPIRED" },
            { status: 401 },
          );
        }
        throw handlerError;
      }
    } catch (error) {
      return handleApiError(error);
    }
  };
}
