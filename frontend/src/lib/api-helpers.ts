// lib/api-helpers.ts
import { isBrowserContext } from "./api-url";
import { sanitizeEndpoint } from "./log-sanitize";
import { createLogger } from "~/lib/logger";
import {
  ApiError,
  apiErrorFromBody,
  unavailableApiError,
  type ProblemBody,
} from "./api-error";

// Logger instance for API helpers
const logger = createLogger({ component: "ApiHelpers" });

/**
 * Type for API response to ensure consistent structure
 */
export interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

/** The error body of an API answer: the RFC 9457 envelope (ADR 0006). */
export type ApiErrorResponse = ProblemBody;

/**
 * Generic domain API error handler: every failure leaves as an ApiError, so
 * the shared display path reads its status and code. An Axios failure keeps
 * the envelope of its response; anything without a response is a server
 * fault.
 * @param error - The caught error
 * @param context - Description of the failed operation
 */
export function handleDomainApiError(error: unknown, context: string): never {
  if (error instanceof ApiError) throw error;
  const response =
    error !== null && typeof error === "object" && "response" in error
      ? (error as { response?: { status?: unknown; data?: unknown } }).response
      : undefined;
  if (response && typeof response.status === "number") {
    throw apiErrorFromBody(
      `Failed to ${context}`,
      response.status,
      response.data,
    );
  }
  throw new ApiError(
    `Failed to ${context}: ${error instanceof Error ? error.message : "Unknown error"}`,
    500,
  );
}

export { isBrowserContext };

/**
 * Options for authenticated fetch requests
 */
interface AuthFetchOptions {
  method?: "GET" | "POST" | "PUT" | "DELETE";
  body?: unknown;
}

/**
 * Perform an authenticated fetch request in browser context. The BFF route
 * authenticates through the session cookie; the browser holds no token
 * (#5005), so a request carries Content-Type only when it has a body.
 * @param url - The URL to fetch
 * @param options - Fetch options including method and body
 * @returns Promise with response data
 * @throws Error if response is not ok
 */
export async function authFetch<T>(
  url: string,
  options: AuthFetchOptions = {},
): Promise<T> {
  const { method = "GET", body } = options;

  const headers: HeadersInit | undefined =
    body === undefined ? undefined : { "Content-Type": "application/json" };

  let response: Response;
  try {
    response = await fetch(url, {
      method,
      credentials: "include",
      headers,
      cache: "no-store", // Prevent caching of dynamic API responses
      ...(body !== undefined && { body: JSON.stringify(body) }),
    });
  } catch (error) {
    // A rejected fetch has no HTTP response or backend error envelope. Keep
    // this transport failure structured so the shared error path can offer a
    // retry instead of treating it as a frontend crash.
    throw unavailableApiError(error);
  }

  if (!response.ok) {
    const bodyText =
      typeof response.text === "function" ? await response.text() : "";
    let payload: unknown;
    try {
      payload = JSON.parse(bodyText) as unknown;
    } catch {
      // Keep the historical statusText message for non-JSON responses.
    }
    throw apiErrorFromBody(
      `API error (${response.status}): ${response.statusText}`,
      response.status,
      payload,
    );
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
interface FetchWithRetryOptions {
  method?: "GET" | "POST" | "PUT" | "DELETE";
  body?: unknown;
  onAuthFailure?: () => Promise<boolean>;
  /** Runs after a successful refresh, before the retry (session re-read). */
  beforeRetry?: () => Promise<unknown>;
}

export interface FetchWithRetryResult<T> {
  response: Response | null;
  data: T | null;
  accessDeniedStatus?: 401 | 403;
}

/**
 * Make an authenticated fetch request with 401 retry logic
 * Handles session refresh and retries the request once on 401. The BFF route
 * reads the session cookie; no Authorization header leaves the browser
 * (#5005).
 * @param url - The URL to fetch
 * @param options - Fetch options including retry handlers
 * @returns Response data or the status of a denied request
 */
export async function fetchWithRetry<T>(
  url: string,
  options: FetchWithRetryOptions = {},
): Promise<FetchWithRetryResult<T>> {
  const { method = "GET", body, onAuthFailure, beforeRetry } = options;

  const makeRequest = async (): Promise<Response> => {
    try {
      return await fetch(url, {
        method,
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        ...(body !== undefined && { body: JSON.stringify(body) }),
      });
    } catch (error) {
      throw unavailableApiError(error);
    }
  };

  // Initial request
  let response = await makeRequest();

  // Handle 401 with retry
  if (response.status === 401 && onAuthFailure) {
    const errorText = await response.text();
    logger.info("token expired, attempting refresh", {
      url: sanitizeEndpoint(url),
      method,
      status: response.status,
      error_text: errorText,
    });

    const refreshSuccessful = await onAuthFailure();

    if (refreshSuccessful) {
      await beforeRetry?.();
      response = await makeRequest();

      if (response.ok) {
        const data = (await response.json()) as T;
        return { response, data };
      }
    } else {
      return { response: null, data: null, accessDeniedStatus: 401 };
    }
  }

  if (!response.ok) {
    const errorText = await response.text();
    // Only 401/403 are expected "access denied" scenarios - return null for graceful handling
    // Other 4xx errors (400 Bad Request, 404 Not Found) indicate bugs and should throw
    if (response.status === 401 || response.status === 403) {
      logger.warn("api access denied", {
        url: sanitizeEndpoint(url),
        method,
        status: response.status,
        error_text: errorText.substring(0, 200),
      });
      return {
        response: null,
        data: null,
        accessDeniedStatus: response.status,
      };
    }
    // All other errors (4xx bugs, 5xx server errors) should throw
    const logContext = {
      url: sanitizeEndpoint(url),
      method,
      status: response.status,
      error_text: errorText.substring(0, 200),
      ...(response.status === 429 && { rate_limited: true }),
    };
    if (response.status === 429) {
      logger.warn("api rate limited", logContext);
    } else {
      logger.error("api error", logContext);
    }
    const parsed = (() => {
      try {
        return JSON.parse(errorText) as unknown;
      } catch {
        return undefined;
      }
    })();
    throw apiErrorFromBody(
      `API error: ${response.status}`,
      response.status,
      parsed,
    );
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
interface BackendRoomType {
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
