// lib/api-helpers.ts
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { isAxiosError } from "axios";
import { auth } from "../server/auth";
import api from "./api";
import { refreshSessionTokensOnServer } from "~/server/auth/token-refresh";

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
}

/**
 * Check if the current session is authenticated
 * @returns NextResponse with error if not authenticated, null if authenticated
 */
export async function checkAuth(): Promise<NextResponse<ApiErrorResponse> | null> {
  const session = await auth();

  if (!session?.user?.token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  return null;
}

/**
 * Make a GET request to the API
 * @param endpoint API endpoint to request
 * @param token Authentication token
 * @returns Promise with the response data
 */
export async function apiGet<T>(endpoint: string, token: string): Promise<T> {
  // In server context, use fetch directly to avoid client-side interceptors
  if (typeof window === "undefined") {
    const { env } = await import("~/env");
    const url = `${env.NEXT_PUBLIC_API_URL}${endpoint}`;

    const executeRequest = async (bearer: string) =>
      fetch(url, {
        headers: {
          Authorization: `Bearer ${bearer}`,
          "Content-Type": "application/json",
        },
      });

    let response = await executeRequest(token);

    if (response.status === 401) {
      const refreshed = await refreshSessionTokensOnServer();
      if (refreshed?.accessToken) {
        response = await executeRequest(refreshed.accessToken);
      }
    }

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`API error (${response.status}): ${errorText}`);
    }

    if (response.status === 204) {
      return {} as T;
    }

    return (await response.json()) as T;
  }

  // In client context, use axios with interceptors
  try {
    const response = await api.get<T>(endpoint, {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    return response.data;
  } catch (error) {
    if (isAxiosError(error)) {
      const status = error.response?.status ?? 500;
      const errorText = error.response?.data
        ? JSON.stringify(error.response.data)
        : error.message;
      throw new Error(`API error (${status}): ${errorText}`);
    }
    throw error;
  }
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
  // In server context, use fetch directly to avoid client-side interceptors
  if (typeof window === "undefined") {
    const { env } = await import("~/env");
    const url = `${env.NEXT_PUBLIC_API_URL}${endpoint}`;

    const executeRequest = async (bearer: string) =>
      fetch(url, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${bearer}`,
          "Content-Type": "application/json",
        },
        body: body ? JSON.stringify(body) : undefined,
      });

    let response = await executeRequest(token);

    if (response.status === 401) {
      const refreshed = await refreshSessionTokensOnServer();
      if (refreshed?.accessToken) {
        response = await executeRequest(refreshed.accessToken);
      }
    }

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`API error (${response.status}): ${errorText}`);
    }

    if (response.status === 204) {
      return {} as T;
    }

    return (await response.json()) as T;
  }

  // In client context, use axios with interceptors
  try {
    const response = await api.post<T>(endpoint, body, {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    return response.data;
  } catch (error) {
    if (isAxiosError(error)) {
      const status = error.response?.status ?? 500;
      const errorText = error.response?.data
        ? JSON.stringify(error.response.data)
        : error.message;
      throw new Error(`API error (${status}): ${errorText}`);
    }
    throw error;
  }
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
  // In server context, use fetch directly to avoid client-side interceptors
  if (typeof window === "undefined") {
    const { env } = await import("~/env");
    const url = `${env.NEXT_PUBLIC_API_URL}${endpoint}`;

    const executeRequest = async (bearer: string) =>
      fetch(url, {
        method: "PUT",
        headers: {
          Authorization: `Bearer ${bearer}`,
          "Content-Type": "application/json",
        },
        body: body ? JSON.stringify(body) : undefined,
      });

    let response = await executeRequest(token);

    if (response.status === 401) {
      const refreshed = await refreshSessionTokensOnServer();
      if (refreshed?.accessToken) {
        response = await executeRequest(refreshed.accessToken);
      }
    }

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`API error (${response.status}): ${errorText}`);
    }

    if (response.status === 204) {
      return {} as T;
    }

    return (await response.json()) as T;
  }

  // In client context, use axios with interceptors
  try {
    const response = await api.put<T>(endpoint, body, {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    return response.data;
  } catch (error) {
    if (isAxiosError(error)) {
      const status = error.response?.status ?? 500;
      const errorText = error.response?.data
        ? JSON.stringify(error.response.data)
        : error.message;
      throw new Error(`API error (${status}): ${errorText}`);
    }
    throw error;
  }
}

/**
 * Make a DELETE request to the API
 * @param endpoint API endpoint to request
 * @param token Authentication token
 * @returns Promise with the response data, or void for 204 No Content responses
 */
export async function apiDelete<T>(
  endpoint: string,
  token: string,
): Promise<T | void> {
  // In server context, use fetch directly to avoid client-side interceptors
  if (typeof window === "undefined") {
    const { env } = await import("~/env");
    const url = `${env.NEXT_PUBLIC_API_URL}${endpoint}`;

    const executeRequest = async (bearer: string) =>
      fetch(url, {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${bearer}`,
          "Content-Type": "application/json",
        },
      });

    let response = await executeRequest(token);

    if (response.status === 401) {
      const refreshed = await refreshSessionTokensOnServer();
      if (refreshed?.accessToken) {
        response = await executeRequest(refreshed.accessToken);
      }
    }

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`API error (${response.status}): ${errorText}`);
    }

    if (response.status === 204) {
      return;
    }

    return (await response.json()) as T;
  }

  // In client context, use axios with interceptors
  try {
    const response = await api.delete<T>(endpoint, {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    // Return void for 204 No Content responses
    if (response.status === 204) {
      return;
    }

    return response.data;
  } catch (error) {
    if (isAxiosError(error)) {
      const status = error.response?.status ?? 500;
      const errorText = error.response?.data
        ? JSON.stringify(error.response.data)
        : error.message;
      throw new Error(`API error (${status}): ${errorText}`);
    }
    throw error;
  }
}

/**
 * Handler for API errors
 * @param error Error object
 * @returns NextResponse with error message and status
 */
export function handleApiError(error: unknown): NextResponse<ApiErrorResponse> {
  console.error("API route error:", error);

  // If it's an Error with a specific status code pattern, extract it
  if (error instanceof Error) {
    const regex = /API error \((\d+)\):/;
    const match = regex.exec(error.message);
    if (match?.[1]) {
      const status = Number.parseInt(match[1], 10);
      return NextResponse.json({ error: error.message }, { status });
    }
  }

  // Default to 500 for unknown errors, but preserve the error message if available
  const errorMessage =
    error instanceof Error ? error.message : "Internal Server Error";
  return NextResponse.json({ error: errorMessage }, { status: 500 });
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

/**
 * Generic domain API error handler
 * Parses API errors and throws standardized error objects with status codes
 * @param error - The caught error
 * @param context - Description of the failed operation
 * @param domain - API domain name for error code prefix (e.g., "STUDENT", "ACTIVITY")
 */
export function handleDomainApiError(
  error: unknown,
  context: string,
  domain: string,
): never {
  // If we have a structured error message with status code
  if (error instanceof Error) {
    const regex = /API error \((\d+)\):/;
    const match = regex.exec(error.message);
    if (match?.[1]) {
      const status = Number.parseInt(match[1], 10);
      const errorMessage = `Failed to ${context}: ${error.message}`;
      throw new Error(
        JSON.stringify({
          status,
          message: errorMessage,
          code: `${domain}_API_ERROR_${status}`,
        }),
      );
    }
  }

  // Default error response
  throw new Error(
    JSON.stringify({
      status: 500,
      message: `Failed to ${context}: ${error instanceof Error ? error.message : "Unknown error"}`,
      code: `${domain}_API_ERROR_UNKNOWN`,
    }),
  );
}

/**
 * Check if running in browser context
 */
export function isBrowserContext(): boolean {
  return globalThis.window !== undefined;
}

/**
 * Options for authenticated fetch requests
 */
export interface AuthFetchOptions {
  method?: "GET" | "POST" | "PUT" | "DELETE";
  body?: unknown;
  token?: string;
}

/**
 * Response from authenticated fetch
 */
export interface AuthFetchResult<T> {
  data: T;
  ok: boolean;
  status: number;
}

/**
 * Build authorization headers for fetch requests
 * @param token - JWT token
 * @param includeContentType - Whether to include Content-Type header
 */
export function buildAuthHeaders(
  token?: string,
  includeContentType = true,
): HeadersInit | undefined {
  if (!token) {
    return includeContentType
      ? { "Content-Type": "application/json" }
      : undefined;
  }
  return {
    Authorization: `Bearer ${token}`,
    ...(includeContentType && { "Content-Type": "application/json" }),
  };
}

/**
 * Perform an authenticated fetch request in browser context
 * @param url - The URL to fetch
 * @param options - Fetch options including method, body, and token
 * @returns Promise with response data
 * @throws Error if response is not ok
 */
export async function authFetch<T>(
  url: string,
  options: AuthFetchOptions = {},
): Promise<T> {
  const { method = "GET", body, token } = options;

  const response = await fetch(url, {
    method,
    credentials: "include",
    headers: buildAuthHeaders(token, method !== "GET" || body !== undefined),
    ...(body !== undefined && { body: JSON.stringify(body) }),
  });

  if (!response.ok) {
    throw new Error(`API error (${response.status}): ${response.statusText}`);
  }

  // Handle 204 No Content
  if (response.status === 204) {
    return {} as T;
  }

  return (await response.json()) as T;
}
