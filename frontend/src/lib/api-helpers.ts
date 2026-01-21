/**
 * API Helpers for Project Phoenix
 *
 * This module provides helper functions for making API requests to the Go backend.
 *
 * BetterAuth Migration Notes:
 * - Server-side requests now forward cookies to the Go backend
 * - The Go backend validates sessions with BetterAuth service
 * - The "token" parameter is now the cookie header string (for compatibility)
 * - Client-side requests use credentials: "include" to send cookies automatically
 */

import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { isAxiosError } from "axios";
import api from "./api";

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
 * HTTP methods supported by the API helpers
 */
type HttpMethod = "GET" | "POST" | "PUT" | "DELETE";

/**
 * Options for server-side fetch requests
 */
interface ServerFetchOptions {
  method: HttpMethod;
  body?: unknown;
  returnVoidOn204?: boolean;
}

/**
 * Check if the current request has a valid session cookie
 * @returns NextResponse with error if not authenticated, null if authenticated
 */
export async function checkAuth(): Promise<NextResponse<ApiErrorResponse> | null> {
  // For BetterAuth, we check for the session cookie
  // This is a lightweight check - actual validation happens on the Go backend
  const { cookies } = await import("next/headers");
  const cookieStore = await cookies();
  const sessionCookie = cookieStore.get("better-auth.session_token");

  if (!sessionCookie?.value) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  return null;
}

/**
 * Server-side fetch with cookie forwarding
 * The "token" parameter is actually the cookie header string (renamed for compatibility)
 */
async function serverFetchWithCookies<T>(
  endpoint: string,
  cookieHeader: string,
  options: ServerFetchOptions,
): Promise<T> {
  const { env } = await import("~/env");
  const url = `${env.NEXT_PUBLIC_API_URL}${endpoint}`;

  const response = await fetch(url, {
    method: options.method,
    headers: {
      Cookie: cookieHeader,
      "Content-Type": "application/json",
    },
    body: options.body ? JSON.stringify(options.body) : undefined,
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`API error (${response.status}): ${errorText}`);
  }

  if (response.status === 204) {
    // DELETE should return void/undefined, others return empty object
    if (options.returnVoidOn204) {
      return undefined as T;
    }
    return {} as T;
  }

  return (await response.json()) as T;
}

/**
 * Client-side axios request with credentials
 * BetterAuth uses cookies, so we include credentials automatically
 */
async function clientAxiosRequest<T>(
  method: HttpMethod,
  endpoint: string,
  _token: string, // Unused - cookies are sent automatically
  body?: unknown,
  returnVoidOn204?: boolean,
): Promise<T> {
  try {
    // axios should send cookies automatically with withCredentials: true
    // This is configured in api.ts
    let response;
    switch (method) {
      case "GET":
        response = await api.get<T>(endpoint);
        break;
      case "POST":
        response = await api.post<T>(endpoint, body);
        break;
      case "PUT":
        response = await api.put<T>(endpoint, body);
        break;
      case "DELETE":
        response = await api.delete<T>(endpoint);
        break;
    }

    // DELETE should return void/undefined for 204
    if (returnVoidOn204 && response.status === 204) {
      return undefined as T;
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
 * Make a GET request to the API
 * @param endpoint API endpoint to request
 * @param cookieHeader Cookie header string (server) or ignored (client)
 * @returns Promise with the response data
 */
export async function apiGet<T>(
  endpoint: string,
  cookieHeader: string,
): Promise<T> {
  if (globalThis.window === undefined) {
    return serverFetchWithCookies<T>(endpoint, cookieHeader, { method: "GET" });
  }
  return clientAxiosRequest<T>("GET", endpoint, cookieHeader);
}

/**
 * Make a POST request to the API
 * @param endpoint API endpoint to request
 * @param cookieHeader Cookie header string (server) or ignored (client)
 * @param body Request body
 * @returns Promise with the response data
 */
export async function apiPost<T, B = unknown>(
  endpoint: string,
  cookieHeader: string,
  body?: B,
): Promise<T> {
  if (globalThis.window === undefined) {
    return serverFetchWithCookies<T>(endpoint, cookieHeader, {
      method: "POST",
      body,
    });
  }
  return clientAxiosRequest<T>("POST", endpoint, cookieHeader, body);
}

/**
 * Make a PUT request to the API
 * @param endpoint API endpoint to request
 * @param cookieHeader Cookie header string (server) or ignored (client)
 * @param body Request body
 * @returns Promise with the response data
 */
export async function apiPut<T, B = unknown>(
  endpoint: string,
  cookieHeader: string,
  body?: B,
): Promise<T> {
  if (globalThis.window === undefined) {
    return serverFetchWithCookies<T>(endpoint, cookieHeader, {
      method: "PUT",
      body,
    });
  }
  return clientAxiosRequest<T>("PUT", endpoint, cookieHeader, body);
}

/**
 * Make a DELETE request to the API
 * @param endpoint API endpoint to request
 * @param cookieHeader Cookie header string (server) or ignored (client)
 * @returns Promise with the response data, or void for 204 No Content responses
 */
export async function apiDelete<T>(
  endpoint: string,
  cookieHeader: string,
): Promise<T | void> {
  if (globalThis.window === undefined) {
    return serverFetchWithCookies<T>(endpoint, cookieHeader, {
      method: "DELETE",
      returnVoidOn204: true,
    });
  }
  return clientAxiosRequest<T>(
    "DELETE",
    endpoint,
    cookieHeader,
    undefined,
    true,
  );
}

/**
 * Handler for API errors
 * @param error Error object
 * @returns NextResponse with error message and status
 */
export function handleApiError(error: unknown): NextResponse<ApiErrorResponse> {
  // If it's an Error with a specific status code pattern, extract it
  if (error instanceof Error) {
    // Match both "API error: 403" and "API error (403):" formats (exactly 3 digits)
    const regex = /API error[:\s(]+(\d{3})/;
    const match = regex.exec(error.message);

    if (match?.[1]) {
      const status = Number.parseInt(match[1], 10);
      // Only log server errors (5xx) to avoid Next.js error overlay for expected 4xx
      if (status >= 500) {
        console.error("API route error:", error);
      } else {
        console.warn("API route error:", error);
      }
      return NextResponse.json({ error: error.message }, { status });
    }
  }

  // Unknown errors are logged as errors and return 500
  console.error("API route error (no status extracted):", error);
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
  token?: string; // Deprecated - cookies are used automatically
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
 * Build headers for requests (now uses cookies, not Authorization header)
 * @param _token - Deprecated, ignored
 */
export function buildAuthHeaders(_token?: string): HeadersInit {
  return {
    "Content-Type": "application/json",
  };
}

/**
 * Build headers for requests that always need Content-Type (POST/PUT with body)
 * @param _token - Deprecated, ignored
 */
export function buildAuthHeadersWithBody(_token?: string): HeadersInit {
  return {
    "Content-Type": "application/json",
  };
}

/**
 * Perform an authenticated fetch request in browser context
 * Uses credentials: "include" to send cookies automatically
 * @param url - The URL to fetch
 * @param options - Fetch options including method, body
 * @returns Promise with response data
 * @throws Error if response is not ok
 */
export async function authFetch<T>(
  url: string,
  options: AuthFetchOptions = {},
): Promise<T> {
  const { method = "GET", body } = options;

  const response = await fetch(url, {
    method,
    credentials: "include", // Send cookies automatically
    headers: {
      "Content-Type": "application/json",
    },
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

/**
 * Options for fetch with retry
 */
export interface FetchWithRetryOptions {
  method?: "GET" | "POST" | "PUT" | "DELETE";
  body?: unknown;
  onAuthFailure?: () => Promise<boolean>;
  getNewToken?: () => Promise<string | undefined>;
}

/**
 * Make an authenticated fetch request with retry logic
 * For BetterAuth, the session cookie is sent automatically
 * @param url - The URL to fetch
 * @param _token - Deprecated, ignored
 * @param options - Fetch options
 * @returns Tuple of [response, data] where response is null if request failed
 */
export async function fetchWithRetry<T>(
  url: string,
  _token: string | undefined,
  options: FetchWithRetryOptions = {},
): Promise<{ response: Response | null; data: T | null }> {
  const { method = "GET", body } = options;

  const response = await fetch(url, {
    method,
    credentials: "include", // Send cookies automatically
    headers: {
      "Content-Type": "application/json",
    },
    ...(body !== undefined && { body: JSON.stringify(body) }),
  });

  if (!response.ok) {
    const errorText = await response.text();
    // 401/403 are expected "access denied" scenarios - return null for graceful handling
    const accessDeniedStatuses = [401, 403];
    if (accessDeniedStatuses.includes(response.status)) {
      console.warn(`API access denied: ${response.status}`, errorText);
      return { response: null, data: null };
    }
    // All other errors (4xx bugs, 5xx server errors) should throw
    console.error(`API error: ${response.status}`, errorText);
    throw new Error(`API error: ${response.status}`);
  }

  const data = (await response.json()) as T;
  return { response, data };
}

/**
 * Type for raw API response that may contain room data
 */
interface RoomApiResponseData {
  id?: number | string;
  name?: string;
  building?: string;
  floor?: number | string;
  capacity?: number | string;
  category?: string;
  color?: string;
  device_id?: string;
  is_occupied?: boolean;
  activity_name?: string;
  group_name?: string;
  supervisor_name?: string;
  student_count?: number;
  created_at?: string;
  updated_at?: string;
  [key: string]: unknown;
}

/**
 * BackendRoom interface for type conversion
 */
export interface BackendRoomType {
  id: number;
  name: string;
  building?: string;
  floor?: number | null;
  capacity?: number | null;
  category?: string | null;
  color?: string | null;
  device_id?: string;
  is_occupied: boolean;
  activity_name?: string;
  group_name?: string;
  supervisor_name?: string;
  student_count?: number;
  created_at: string;
  updated_at: string;
}

/**
 * Parse a value that may be number, string, or undefined into a number
 * @param value - The value to parse
 * @param defaultValue - Default if parsing fails (default: 0)
 */
function parseNumericField(
  value: number | string | undefined,
  defaultValue = 0,
): number {
  if (typeof value === "number") return value;
  if (typeof value === "string") return Number.parseInt(value, 10);
  return defaultValue;
}

/**
 * Parse a value into a required string (empty string as fallback)
 * @param value - The value to parse
 */
function parseRequiredString(value: string | undefined): string {
  return typeof value === "string" ? value : "";
}

/**
 * Parse a value into an optional string (undefined as fallback)
 * @param value - The value to parse
 */
function parseOptionalString(value: string | undefined): string | undefined {
  return typeof value === "string" ? value : undefined;
}

/**
 * Parse a value into an optional number (undefined as fallback)
 * @param value - The value to parse
 */
function parseOptionalNumber(value: number | undefined): number | undefined {
  return typeof value === "number" ? value : undefined;
}

/**
 * Safely convert a raw API response to BackendRoom type
 * Handles type coercion for all fields with proper defaults
 * @param responseData - Raw response data from API
 * @returns Properly typed BackendRoom object
 */
export function convertToBackendRoom(
  responseData: RoomApiResponseData,
): BackendRoomType {
  return {
    id: parseNumericField(responseData.id),
    name: parseRequiredString(responseData.name),
    building: parseOptionalString(responseData.building),
    floor: parseNumericField(responseData.floor),
    capacity: parseNumericField(responseData.capacity),
    category: parseRequiredString(responseData.category),
    color: parseRequiredString(responseData.color),
    device_id: parseOptionalString(responseData.device_id),
    is_occupied: Boolean(responseData.is_occupied),
    activity_name: parseOptionalString(responseData.activity_name),
    group_name: parseOptionalString(responseData.group_name),
    supervisor_name: parseOptionalString(responseData.supervisor_name),
    student_count: parseOptionalNumber(responseData.student_count),
    created_at: parseRequiredString(responseData.created_at),
    updated_at: parseRequiredString(responseData.updated_at),
  };
}
