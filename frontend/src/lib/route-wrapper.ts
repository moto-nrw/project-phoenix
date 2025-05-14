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
    { params }: { params: Record<string, unknown> }
  ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse>> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return NextResponse.json(
          { error: "Unauthorized" },
          { status: 401 }
        );
      }

      const data = await handler(request, session.user.token, params);
      return NextResponse.json(data);
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
    { params }: { params: Record<string, unknown> }
  ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse>> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return NextResponse.json(
          { error: "Unauthorized" },
          { status: 401 }
        );
      }

      const body = await request.json() as B;
      const data = await handler(request, body, session.user.token, params);
      return NextResponse.json(data);
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
    { params }: { params: Record<string, unknown> }
  ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse>> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return NextResponse.json(
          { error: "Unauthorized" },
          { status: 401 }
        );
      }

      const body = await request.json() as B;
      const data = await handler(request, body, session.user.token, params);
      return NextResponse.json(data);
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
    { params }: { params: Record<string, unknown> }
  ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse>> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return NextResponse.json(
          { error: "Unauthorized" },
          { status: 401 }
        );
      }

      const data = await handler(request, session.user.token, params);
      return NextResponse.json(data);
    } catch (error) {
      return handleApiError(error);
    }
  };
}