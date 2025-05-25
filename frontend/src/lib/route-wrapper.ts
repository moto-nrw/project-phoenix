import type { NextRequest } from "next/server";
  import { NextResponse } from "next/server";
  import { auth } from "../server/auth";
  import type { ApiErrorResponse, ApiResponse } from "./api-helpers";
  import { handleApiError } from "./api-helpers";

  /**
   * Wrapper function for handling GET API routes
   * @param handler Function that handles the API request
   * @returns Response from the handler or error response
   */
  export function createGetHandler<T>(
    handler: (
      request: NextRequest,
      token: string,
      params: Record<string, unknown>
    ) => Promise<T>
  ) {
    return async (
      request: NextRequest,
      context: { params: Promise<Record<string, string | string[] | undefined>> }
    ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse | T>> => {
      try {
        const session = await auth();

        if (!session?.user?.token) {
          return NextResponse.json(
            { error: "Unauthorized" },
            { status: 401 }
          );
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
        const pathParts = url.pathname.split('/');

        // Try to extract ID from URL path parts if not already set
        if (!safeParams.id) {
          const potentialIds = pathParts.filter(part => /^\d+$/.test(part));
          if (potentialIds.length > 0) {
            // Use the last numeric part as ID
            safeParams.id = potentialIds[potentialIds.length - 1];
          }
        }

        // Extract search params
        url.searchParams.forEach((value, key) => {
          safeParams[key] = value;
        });

        const data = await handler(request, session.user.token, safeParams);

        // For the rooms endpoint, we need to pass the raw data directly
        if (request.nextUrl.pathname === '/api/rooms') {
          return NextResponse.json(data);
        }

        // Wrap the response in ApiResponse format if it's not already
        const response: ApiResponse<T> = typeof data === 'object' && data !== null && 'success' in data
          ? (data as unknown as ApiResponse<T>)
          : { success: true, message: "Success", data };

        return NextResponse.json(response);
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
      params: Record<string, unknown>
    ) => Promise<T>
  ) {
    return async (
      request: NextRequest,
      context: { params: Promise<Record<string, string | string[] | undefined>> }
    ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse | T>> => {
      try {
        const session = await auth();

        if (!session?.user?.token) {
          return NextResponse.json(
            { error: "Unauthorized" },
            { status: 401 }
          );
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
        const pathParts = url.pathname.split('/');

        // Try to extract ID from URL path parts if not already set
        if (!safeParams.id) {
          const potentialIds = pathParts.filter(part => /^\d+$/.test(part));
          if (potentialIds.length > 0) {
            // Use the last numeric part as ID
            safeParams.id = potentialIds[potentialIds.length - 1];
          }
        }

        // Extract search params
        url.searchParams.forEach((value, key) => {
          safeParams[key] = value;
        });

        let body: B;
        try {
          const text = await request.text();
          body = text ? JSON.parse(text) as B : {} as B;
        } catch {
          body = {} as B;
        }
        const data = await handler(request, body, session.user.token, safeParams);

        // Wrap the response in ApiResponse format if it's not already
        const response: ApiResponse<T> = typeof data === 'object' && data !== null && 'success' in data
          ? (data as unknown as ApiResponse<T>)
          : { success: true, message: "Success", data };

        return NextResponse.json(response);
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
      params: Record<string, unknown>
    ) => Promise<T>
  ) {
    return async (
      request: NextRequest,
      context: { params: Promise<Record<string, string | string[] | undefined>> }
    ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse | T>> => {
      try {
        const session = await auth();

        if (!session?.user?.token) {
          return NextResponse.json(
            { error: "Unauthorized" },
            { status: 401 }
          );
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
        const pathParts = url.pathname.split('/');

        // Try to extract ID from URL path parts if not already set
        if (!safeParams.id) {
          const potentialIds = pathParts.filter(part => /^\d+$/.test(part));
          if (potentialIds.length > 0) {
            // Use the last numeric part as ID
            safeParams.id = potentialIds[potentialIds.length - 1];
          }
        }

        // Extract search params
        url.searchParams.forEach((value, key) => {
          safeParams[key] = value;
        });

        let body: B;
        try {
          const text = await request.text();
          body = text ? JSON.parse(text) as B : {} as B;
        } catch {
          body = {} as B;
        }
        const data = await handler(request, body, session.user.token, safeParams);

        // Wrap the response in ApiResponse format if it's not already
        const response: ApiResponse<T> = typeof data === 'object' && data !== null && 'success' in data
          ? (data as unknown as ApiResponse<T>)
          : { success: true, message: "Success", data };

        return NextResponse.json(response);
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
      params: Record<string, unknown>
    ) => Promise<T>
  ) {
    return async (
      request: NextRequest,
      context: { params: Promise<Record<string, string | string[] | undefined>> }
    ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse | T>> => {
      try {
        const session = await auth();

        if (!session?.user?.token) {
          return NextResponse.json(
            { error: "Unauthorized" },
            { status: 401 }
          );
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
        const pathParts = url.pathname.split('/');

        // Try to extract ID from URL path parts if not already set
        if (!safeParams.id) {
          const potentialIds = pathParts.filter(part => /^\d+$/.test(part));
          if (potentialIds.length > 0) {
            // Use the last numeric part as ID
            safeParams.id = potentialIds[potentialIds.length - 1];
          }
        }

        // Extract search params
        url.searchParams.forEach((value, key) => {
          safeParams[key] = value;
        });

        const data = await handler(request, session.user.token, safeParams);

        // For delete operations with no content, return 204 status
        if (data === null || data === undefined) {
          return new NextResponse(null, { status: 204 });
        }

        // Wrap the response in ApiResponse format if it's not already
        const response: ApiResponse<T> = typeof data === 'object' && data !== null && 'success' in data
          ? (data as unknown as ApiResponse<T>)
          : { success: true, message: "Success", data };

        return NextResponse.json(response);
      } catch (error) {
        return handleApiError(error);
      }
    };
  }