import axios from "axios";
import type { AxiosError, AxiosRequestConfig, AxiosResponse } from "axios";
import { env } from "~/env";
import { sanitizeEndpoint } from "~/lib/log-sanitize";
import { createLogger } from "~/lib/logger";
import { clearSessionCache, getCachedSession } from "~/lib/session-cache";

/**
 * Extended request config with retry tracking properties
 */
interface RetryableRequestConfig extends AxiosRequestConfig {
  _retry?: boolean;
  _retryCount?: number;
}

// Logger instance for API client
const logger = createLogger({ component: "ApiClient" });

// Create an Axios instance
// oxlint-disable-next-line import/no-named-as-default-member -- Axios exposes create on the default export at runtime, but not as a TypeScript named export.
const createAxios = axios["create"];
const api = createAxios({
  baseURL: env.NEXT_PUBLIC_API_URL,
  headers: {
    "Content-Type": "application/json",
  },
  // Important: Include credentials with every request to ensure cookies are sent
  withCredentials: true,
});

// Add a request interceptor to include the auth token
// Note: This interceptor only runs in client-side code
api.interceptors.request.use(
  async (config) => {
    // Only try to get session if we're in the browser. getCachedSession
    // deduplicates concurrent session lookups — every raw getSession() call is
    // its own network round trip to /api/auth/session (#2123).
    if (globalThis.window !== undefined) {
      const session = await getCachedSession();

      // If there's a token, add it to the headers
      if (session?.user?.token) {
        config.headers.Authorization = `Bearer ${session.user.token}`;
        logger.debug("token injected in request", {
          method: config.method?.toUpperCase(),
          url: config.url,
          has_token: true,
        });
      } else {
        logger.debug("no token available for request", {
          method: config.method?.toUpperCase(),
          url: config.url,
        });
      }
    }

    return config;
  },
  (error: Error) => {
    logger.error("request interceptor error", {
      error: error.message,
    });
    return Promise.reject(error);
  },
);

// Track ongoing refresh attempts to prevent multiple simultaneous refreshes
let isRefreshing = false;
let lastRefreshedToken: string | null = null;
let refreshSubscribers: {
  resolve: (token: string) => void;
  reject: (error: Error) => void;
}[] = [];

// Subscribe to token refresh completion
const subscribeTokenRefresh = (callbacks: {
  resolve: (token: string) => void;
  reject: (error: Error) => void;
}) => {
  refreshSubscribers.push(callbacks);
};

// Notify all subscribers when refresh is complete
const onTokenRefreshed = (token: string) => {
  lastRefreshedToken = token;
  refreshSubscribers.forEach((subscriber) => subscriber.resolve(token));
  refreshSubscribers = [];
};

// Reject all subscribers when refresh fails
const onTokenRefreshFailed = (error: Error) => {
  refreshSubscribers.forEach((subscriber) => subscriber.reject(error));
  refreshSubscribers = [];
};

// Helper: Redirect to login page (browser only)
function redirectToLogin(): void {
  if (globalThis.window !== undefined) {
    globalThis.window.location.href = "/";
  }
}

// Helper: Queue request for token refresh completion
function queueRequestForRefresh(
  originalRequest: AxiosRequestConfig,
  _callerId: string,
): Promise<AxiosResponse> {
  return new Promise((resolve, reject) => {
    subscribeTokenRefresh({
      resolve: (token: string) => {
        originalRequest.headers ??= {};
        originalRequest.headers.Authorization = `Bearer ${token}`;
        resolve(api(originalRequest));
      },
      reject,
    });
  });
}

// Helper: Attempt client-side token refresh
async function attemptClientSideRefresh(
  originalRequest: AxiosRequestConfig,
): Promise<AxiosResponse | null> {
  const { handleAuthFailure } = await import("./auth-failure");
  const refreshSuccessful = await handleAuthFailure();

  if (!refreshSuccessful || !originalRequest.headers) {
    return null;
  }

  // The refresh rotated the tokens. Drop the cached session BEFORE reading it
  // again: the retry below re-runs the request interceptor, which reads
  // getCachedSession() — served the stale pre-refresh token, the retry would
  // 401 again and fail hard (_retry is already set). Repopulating the cache
  // here also hands the fresh token to every queued subscriber.
  clearSessionCache();
  const session = await getCachedSession();

  if (!session?.user?.token) {
    return null;
  }

  onTokenRefreshed(session.user.token);
  originalRequest.headers.Authorization = `Bearer ${session.user.token}`;
  return api(originalRequest);
}

// Add a response interceptor to handle common errors
api.interceptors.response.use(
  (response: AxiosResponse) => response,
  async (error: AxiosError) => {
    const originalRequest = error.config as RetryableRequestConfig | undefined;

    // Log non-401 errors
    if (error.response?.status !== 401) {
      logger.error("api request failed", {
        method: originalRequest?.method?.toUpperCase(),
        url: sanitizeEndpoint(originalRequest?.url ?? ""),
        status: error.response?.status,
        error: error.message,
      });
      throw error;
    }

    // Only handle 401 errors that haven't been retried
    if (!originalRequest || originalRequest._retry) {
      throw error;
    }

    const callerId = `axios-interceptor-${Date.now()}-${crypto.randomUUID().slice(0, 8)}`;
    originalRequest._retry = true;
    originalRequest._retryCount = (originalRequest._retryCount ?? 0) + 1;

    logger.info("token expired, attempting refresh", {
      method: originalRequest.method?.toUpperCase(),
      url: sanitizeEndpoint(originalRequest.url ?? ""),
      retry_count: originalRequest._retryCount,
      caller_id: callerId,
    });

    // Limit retry attempts
    if (originalRequest._retryCount > 3) {
      logger.warn("max token refresh retries reached", {
        method: originalRequest.method?.toUpperCase(),
        url: sanitizeEndpoint(originalRequest.url ?? ""),
        retry_count: originalRequest._retryCount,
        action: "redirecting to login",
      });
      redirectToLogin();
      throw error;
    }

    // Queue request if refresh is already in progress
    if (isRefreshing) {
      logger.debug("token refresh in progress, queueing request", {
        caller_id: callerId,
        url: originalRequest.url,
      });
      return queueRequestForRefresh(originalRequest, callerId);
    }

    isRefreshing = true;
    lastRefreshedToken = null;

    try {
      if (globalThis.window === undefined) {
        logger.error("axios token refresh attempted from server context", {
          caller_id: callerId,
        });
        throw error;
      }

      // Client-side refresh
      logger.info("attempting client-side token refresh", {
        caller_id: callerId,
      });
      const result = await attemptClientSideRefresh(originalRequest);
      if (result) {
        logger.info("client-side token refresh successful", {
          caller_id: callerId,
        });
        return result;
      }

      logger.warn("client-side token refresh failed, redirecting", {
        caller_id: callerId,
      });
      redirectToLogin();
    } finally {
      isRefreshing = false;
      if (refreshSubscribers.length > 0) {
        if (lastRefreshedToken) {
          // Late arrivals queued after onTokenRefreshed but before finally —
          // the refresh succeeded, so resolve them with the valid token.
          logger.info("resolving_late_queued_requests", {
            queued_count: refreshSubscribers.length,
          });
          onTokenRefreshed(lastRefreshedToken);
        } else {
          logger.warn("token_refresh_failed_rejecting_queued_requests", {
            queued_count: refreshSubscribers.length,
          });
          onTokenRefreshFailed(new Error("Token refresh failed"));
        }
      }
      lastRefreshedToken = null;
    }

    throw error;
  },
);

export default api;
