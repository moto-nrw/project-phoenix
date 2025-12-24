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

// Helper: Client-side API GET request using axios
async function apiGetClient<T>(endpoint: string, token: string): Promise<T> {
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
 * Make a GET request to the API
 * @param endpoint API endpoint to request
 * @param token Authentication token
 * @returns Promise with the response data
 */
export async function apiGet<T>(endpoint: string, token: string): Promise<T> {
  // In server context, use fetch directly to avoid client-side interceptors
  if (typeof globalThis.window === "undefined") {
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
  return apiGetClient(endpoint, token);
}

// Helper: Client-side API POST request using axios
async function apiPostClient<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
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
  if (typeof globalThis.window === "undefined") {
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
  return apiPostClient(endpoint, token, body);
}

// Helper: Client-side API PUT request using axios
async function apiPutClient<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
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
  if (typeof globalThis.window === "undefined") {
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
  return apiPutClient(endpoint, token, body);
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
  if (typeof globalThis.window === "undefined") {
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
 * Matches original behavior: always includes Content-Type when token is present
 * @param token - JWT token
 */
export function buildAuthHeaders(token?: string): HeadersInit | undefined {
  if (!token) {
    return undefined;
  }
  return {
    Authorization: `Bearer ${token}`,
    "Content-Type": "application/json",
  };
}

/**
 * Build headers for requests that always need Content-Type (POST/PUT with body)
 * @param token - JWT token
 */
export function buildAuthHeadersWithBody(token?: string): HeadersInit {
  return {
    "Content-Type": "application/json",
    ...(token && { Authorization: `Bearer ${token}` }),
  };
}

/**
 * Perform an authenticated fetch request in browser context
 * Matches original fetch pattern exactly:
 * - GET/DELETE with token: Authorization + Content-Type headers
 * - GET/DELETE without token: no headers
 * - POST/PUT: Always Content-Type, Authorization if token present
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

  // Match original header behavior:
  // - POST/PUT with body: always include Content-Type, add Auth if token
  // - GET/DELETE: include both headers only if token present
  const headers =
    body === undefined
      ? buildAuthHeaders(token)
      : buildAuthHeadersWithBody(token);

  const response = await fetch(url, {
    method,
    credentials: "include",
    headers,
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
 * Make an authenticated fetch request with 401 retry logic
 * Handles token refresh and retries the request once on 401
 * @param url - The URL to fetch
 * @param token - Current auth token
 * @param options - Fetch options including retry handlers
 * @returns Tuple of [response, data] where response is null if request failed after retry
 */
export async function fetchWithRetry<T>(
  url: string,
  token: string | undefined,
  options: FetchWithRetryOptions = {},
): Promise<{ response: Response | null; data: T | null }> {
  const { method = "GET", body, onAuthFailure, getNewToken } = options;

  const makeRequest = async (
    authToken: string | undefined,
  ): Promise<Response> => {
    const headers = authToken
      ? {
          Authorization: `Bearer ${authToken}`,
          "Content-Type": "application/json",
        }
      : undefined;

    return fetch(url, {
      method,
      credentials: "include",
      headers,
      ...(body !== undefined && { body: JSON.stringify(body) }),
    });
  };

  // Initial request
  const response = await makeRequest(token);

  // Handle 401 with retry
  if (response.status === 401 && onAuthFailure && getNewToken) {
    const errorText = await response.text();
    console.error(`API error: ${response.status}`, errorText);

    const refreshSuccessful = await onAuthFailure();

    if (refreshSuccessful) {
      const newToken = await getNewToken();
      const retryResponse = await makeRequest(newToken);

      if (retryResponse.ok) {
        const data = (await retryResponse.json()) as T;
        return { response: retryResponse, data };
      }
    }

    return { response: null, data: null };
  }

  if (!response.ok) {
    const errorText = await response.text();
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
