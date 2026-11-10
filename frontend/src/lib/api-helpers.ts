// lib/api-helpers.ts
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
  /** The parsed error envelope of the refusal: code, details, instance. */
  accessDeniedBody?: unknown;
}

function parseErrorText(text: string): unknown {
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return undefined;
  }
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
      return {
        response: null,
        data: null,
        accessDeniedStatus: 401,
        accessDeniedBody: parseErrorText(errorText),
      };
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
        accessDeniedBody: parseErrorText(errorText),
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
    throw apiErrorFromBody(
      `API error: ${response.status}`,
      response.status,
      parseErrorText(errorText),
    );
  }

  const data = (await response.json()) as T;
  return { response, data };
}
