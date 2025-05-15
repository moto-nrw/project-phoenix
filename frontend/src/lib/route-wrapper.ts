// lib/route-wrapper.ts
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
    _context: { params: Record<string, unknown> }
  ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse>> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return NextResponse.json(
          { error: "Unauthorized" },
          { status: 401 }
        );
      }

      // In Next.js App Router, dynamic params need to be properly handled to avoid
      // "params should be awaited before using its properties" error
      const safeParams: Record<string, unknown> = {};
      
      // Create a new object with only the properties we need
      // This avoids direct property access on context.params which can trigger the error
      // Extract ID from URL path instead of accessing context.params directly
      const pathParts = request.nextUrl.pathname.split('/');
      const lastPathPart = pathParts[pathParts.length - 1];
      // Detect if the last path part is a numeric ID
      const id = lastPathPart && /^\d+$/.test(lastPathPart) ? lastPathPart : undefined;
      
      if (id) {
        safeParams.id = id;
      }
      
      const data = await handler(request, session.user.token, safeParams);
      
      // For the rooms endpoint, we need to pass the raw data directly
      // This is a special case for the rooms endpoint which needs to return the array directly
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
    _context: { params: Record<string, unknown> }
  ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse>> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return NextResponse.json(
          { error: "Unauthorized" },
          { status: 401 }
        );
      }

      // In Next.js App Router, dynamic params need to be properly handled to avoid
      // "params should be awaited before using its properties" error
      const safeParams: Record<string, unknown> = {};
      
      // Create a new object with only the properties we need
      // This avoids direct property access on context.params which can trigger the error
      // Extract ID from URL path instead of accessing context.params directly
      const pathParts = request.nextUrl.pathname.split('/');
      const lastPathPart = pathParts[pathParts.length - 1];
      // Detect if the last path part is a numeric ID
      const id = lastPathPart && /^\d+$/.test(lastPathPart) ? lastPathPart : undefined;
      
      if (id) {
        safeParams.id = id;
      }

      const body = await request.json() as B;
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
    _context: { params: Record<string, unknown> }
  ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse>> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return NextResponse.json(
          { error: "Unauthorized" },
          { status: 401 }
        );
      }

      // In Next.js App Router, dynamic params need to be properly handled to avoid
      // "params should be awaited before using its properties" error
      const safeParams: Record<string, unknown> = {};
      
      // Create a new object with only the properties we need
      // This avoids direct property access on context.params which can trigger the error
      // Extract ID from URL path instead of accessing context.params directly
      const pathParts = request.nextUrl.pathname.split('/');
      const lastPathPart = pathParts[pathParts.length - 1];
      // Detect if the last path part is a numeric ID
      const id = lastPathPart && /^\d+$/.test(lastPathPart) ? lastPathPart : undefined;
      
      if (id) {
        safeParams.id = id;
      }

      const body = await request.json() as B;
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
    _context: { params: Record<string, unknown> }
  ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse>> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return NextResponse.json(
          { error: "Unauthorized" },
          { status: 401 }
        );
      }

      // In Next.js App Router, dynamic params need to be properly handled to avoid
      // "params should be awaited before using its properties" error
      const safeParams: Record<string, unknown> = {};
      
      // Create a new object with only the properties we need
      // This avoids direct property access on context.params which can trigger the error
      // Extract ID from URL path instead of accessing context.params directly
      const pathParts = request.nextUrl.pathname.split('/');
      const lastPathPart = pathParts[pathParts.length - 1];
      // Detect if the last path part is a numeric ID
      const id = lastPathPart && /^\d+$/.test(lastPathPart) ? lastPathPart : undefined;
      
      if (id) {
        safeParams.id = id;
      }

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