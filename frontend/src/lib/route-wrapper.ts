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

        try {
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
        } catch (handlerError) {
          // Check if it's a 401 error from the backend
          if (handlerError instanceof Error && handlerError.message.includes("API error (401)")) {
            const callerId = `route-wrapper-get-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
            console.log(`\n[${callerId}] Route wrapper GET: Caught 401 error`);
            // Try to get updated session in case it was refreshed
            const updatedSession = await auth();
            
            // If we have an updated token, retry once
            if (updatedSession?.user?.token && updatedSession.user.token !== session.user.token) {
              console.log(`[${callerId}] Token was refreshed by another process, retrying request with new token`);
              try {
                const retryData = await handler(request, updatedSession.user.token, safeParams);
                
                // For the rooms endpoint, we need to pass the raw data directly
                if (request.nextUrl.pathname === '/api/rooms') {
                  return NextResponse.json(retryData);
                }
                
                // Wrap the response in ApiResponse format if it's not already
                const retryResponse: ApiResponse<T> = typeof retryData === 'object' && retryData !== null && 'success' in retryData
                  ? (retryData as unknown as ApiResponse<T>)
                  : { success: true, message: "Success", data: retryData };
                
                return NextResponse.json(retryResponse);
              } catch {
                // If retry also fails, return 401
                return NextResponse.json(
                  { error: "Token expired", code: "TOKEN_EXPIRED" },
                  { status: 401 }
                );
              }
            }
            
            // Return 401 to client so it can handle token refresh
            return NextResponse.json(
              { error: "Token expired", code: "TOKEN_EXPIRED" },
              { status: 401 }
            );
          }
          throw handlerError;
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
        
        try {
          const data = await handler(request, body, session.user.token, safeParams);

          // Wrap the response in ApiResponse format if it's not already
          const response: ApiResponse<T> = typeof data === 'object' && data !== null && 'success' in data
            ? (data as unknown as ApiResponse<T>)
            : { success: true, message: "Success", data };

          return NextResponse.json(response);
        } catch (handlerError) {
          // Check if it's a 401 error from the backend
          if (handlerError instanceof Error && handlerError.message.includes("API error (401)")) {
            // Try to get updated session in case it was refreshed
            const updatedSession = await auth();
            
            // If we have an updated token, retry once
            if (updatedSession?.user?.token && updatedSession.user.token !== session.user.token) {
              console.log("Token was refreshed, retrying POST request with new token");
              try {
                const retryData = await handler(request, body, updatedSession.user.token, safeParams);
                
                // Wrap the response in ApiResponse format if it's not already
                const retryResponse: ApiResponse<T> = typeof retryData === 'object' && retryData !== null && 'success' in retryData
                  ? (retryData as unknown as ApiResponse<T>)
                  : { success: true, message: "Success", data: retryData };
                
                return NextResponse.json(retryResponse);
              } catch {
                // If retry also fails, return 401
                return NextResponse.json(
                  { error: "Token expired", code: "TOKEN_EXPIRED" },
                  { status: 401 }
                );
              }
            }
            
            // Return 401 to client so it can handle token refresh
            return NextResponse.json(
              { error: "Token expired", code: "TOKEN_EXPIRED" },
              { status: 401 }
            );
          }
          throw handlerError;
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
        
        try {
          const data = await handler(request, body, session.user.token, safeParams);

          // Wrap the response in ApiResponse format if it's not already
          const response: ApiResponse<T> = typeof data === 'object' && data !== null && 'success' in data
            ? (data as unknown as ApiResponse<T>)
            : { success: true, message: "Success", data };

          return NextResponse.json(response);
        } catch (handlerError) {
          // Check if it's a 401 error from the backend
          if (handlerError instanceof Error && handlerError.message.includes("API error (401)")) {
            // Try to get updated session in case it was refreshed
            const updatedSession = await auth();
            
            // If we have an updated token, retry once
            if (updatedSession?.user?.token && updatedSession.user.token !== session.user.token) {
              console.log("Token was refreshed, retrying POST request with new token");
              try {
                const retryData = await handler(request, body, updatedSession.user.token, safeParams);
                
                // Wrap the response in ApiResponse format if it's not already
                const retryResponse: ApiResponse<T> = typeof retryData === 'object' && retryData !== null && 'success' in retryData
                  ? (retryData as unknown as ApiResponse<T>)
                  : { success: true, message: "Success", data: retryData };
                
                return NextResponse.json(retryResponse);
              } catch {
                // If retry also fails, return 401
                return NextResponse.json(
                  { error: "Token expired", code: "TOKEN_EXPIRED" },
                  { status: 401 }
                );
              }
            }
            
            // Return 401 to client so it can handle token refresh
            return NextResponse.json(
              { error: "Token expired", code: "TOKEN_EXPIRED" },
              { status: 401 }
            );
          }
          throw handlerError;
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

        try {
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
        } catch (handlerError) {
          // Check if it's a 401 error from the backend
          if (handlerError instanceof Error && handlerError.message.includes("API error (401)")) {
            // Try to get updated session in case it was refreshed
            const updatedSession = await auth();
            
            // If we have an updated token, retry once
            if (updatedSession?.user?.token && updatedSession.user.token !== session.user.token) {
              console.log("Token was refreshed, retrying DELETE request with new token");
              try {
                const retryData = await handler(request, updatedSession.user.token, safeParams);
                
                // For delete operations with no content, return 204 status
                if (retryData === null || retryData === undefined) {
                  return new NextResponse(null, { status: 204 });
                }
                
                // Wrap the response in ApiResponse format if it's not already
                const retryResponse: ApiResponse<T> = typeof retryData === 'object' && retryData !== null && 'success' in retryData
                  ? (retryData as unknown as ApiResponse<T>)
                  : { success: true, message: "Success", data: retryData };
                
                return NextResponse.json(retryResponse);
              } catch {
                // If retry also fails, return 401
                return NextResponse.json(
                  { error: "Token expired", code: "TOKEN_EXPIRED" },
                  { status: 401 }
                );
              }
            }
            
            // Return 401 to client so it can handle token refresh
            return NextResponse.json(
              { error: "Token expired", code: "TOKEN_EXPIRED" },
              { status: 401 }
            );
          }
          throw handlerError;
        }
      } catch (error) {
        return handleApiError(error);
      }
    };
  }