import axios from "axios";
import type { AxiosError, AxiosRequestConfig, AxiosResponse } from "axios";
import { clientEnv } from "~/env.client";
import { sanitizeEndpoint } from "~/lib/log-sanitize";
import { createLogger } from "~/lib/logger";
import {
  rateLimitBlockedError,
  recordRateLimit,
} from "~/lib/rate-limit-backoff";
import { hasSessionAccess } from "~/lib/session-access";
import {
  boundSessionAccount,
  isOwnApiUrl,
  SESSION_ACCOUNT_HEADER,
} from "~/lib/session-account";
import { clearSessionCache, getCachedSession } from "~/lib/session-cache";
import { sessionBarrierSignal } from "~/lib/session-end-barrier";

/**
 * Extended request config with retry tracking properties
 */
interface RetryableRequestConfig extends AxiosRequestConfig {
  _retry?: boolean;
  _retryCount?: number;
  /** The caller's own signal, kept apart from the session-end barrier's. */
  _callerSignal?: AbortSignal | null;
}

// Logger instance for API client
const logger = createLogger({ component: "ApiClient" });

// Create an Axios instance
// oxlint-disable-next-line import/no-named-as-default-member -- Axios exposes create on the default export at runtime, but not as a TypeScript named export.
const createAxios = axios["create"];
const api = createAxios({
  baseURL: clientEnv.NEXT_PUBLIC_API_URL,
  headers: {
    "Content-Type": "application/json",
  },
  // Important: Include credentials with every request to ensure cookies are sent
  withCredentials: true,
});

// Request interceptor: rate-limit backoff and, for this tab's own BFF, the
// account the tab holds data for (#5183). The browser holds no token
// (#5005); BFF routes authenticate through the session cookie. Other
// origins never get the header: it would need a CORS preflight.
api.interceptors.request.use(
  async (config) => {
    const rateLimitError = rateLimitBlockedError(config.method);
    if (rateLimitError) throw rateLimitError;
    if (!isOwnApiUrl(api.getUri(config))) return config;
    // A logout holds and aborts requests to the own BFF until the session
    // cookie is gone (#5399); XHR sets cookies like fetch does.
    const retryable = config as typeof config & RetryableRequestConfig;
    if (retryable._callerSignal === undefined) {
      retryable._callerSignal =
        (config.signal as AbortSignal | undefined) ?? null;
    }
    config.signal = await sessionBarrierSignal(
      retryable._callerSignal ?? undefined,
    );
    const accountId = boundSessionAccount();
    if (accountId !== null) {
      config.headers.set(SESSION_ACCOUNT_HEADER, accountId);
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
let refreshSucceeded = false;
let refreshSubscribers: {
  resolve: () => void;
  reject: (error: Error) => void;
}[] = [];

// Subscribe to token refresh completion
const subscribeTokenRefresh = (callbacks: {
  resolve: () => void;
  reject: (error: Error) => void;
}) => {
  refreshSubscribers.push(callbacks);
};

// Notify all subscribers when refresh is complete
const onTokenRefreshed = () => {
  refreshSucceeded = true;
  refreshSubscribers.forEach((subscriber) => subscriber.resolve());
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
      resolve: () => resolve(api(originalRequest)),
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

  if (!refreshSuccessful) {
    return null;
  }

  // The refresh rotated the tokens inside the session cookie. Drop the cached
  // session BEFORE reading it again: the session lookup persists the rotated
  // cookie, and a stale cached session would hide a refresh that ended the
  // session. The retry then goes out with the fresh cookie.
  clearSessionCache();
  const session = await getCachedSession();

  if (!hasSessionAccess(session)) {
    return null;
  }

  onTokenRefreshed();
  return api(originalRequest);
}

// Add a response interceptor to handle common errors
api.interceptors.response.use(
  (response: AxiosResponse) => response,
  async (error: AxiosError) => {
    const originalRequest = error.config as RetryableRequestConfig | undefined;

    // The session-end barrier stopped a read before its answer could set a
    // cookie (#5399). Ask again once it opens; the request interceptor waits.
    if (
      error.code === "ERR_CANCELED" &&
      originalRequest &&
      originalRequest._callerSignal !== undefined &&
      !originalRequest._callerSignal?.aborted &&
      (originalRequest.method ?? "get").toLowerCase() === "get"
    ) {
      return api(originalRequest);
    }

    if (error.response?.status === 429) {
      const retryAfter = error.response.headers?.["retry-after"];
      recordRateLimit(
        typeof retryAfter === "string" ? retryAfter : null,
        originalRequest?.method,
      );
      throw error;
    }

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
        url: sanitizeEndpoint(originalRequest.url ?? ""),
      });
      return queueRequestForRefresh(originalRequest, callerId);
    }

    isRefreshing = true;
    refreshSucceeded = false;

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
        if (refreshSucceeded) {
          // Late arrivals queued after onTokenRefreshed but before finally —
          // the refresh succeeded, so retry them with the fresh cookie.
          logger.info("resolving_late_queued_requests", {
            queued_count: refreshSubscribers.length,
          });
          onTokenRefreshed();
        } else {
          logger.warn("token_refresh_failed_rejecting_queued_requests", {
            queued_count: refreshSubscribers.length,
          });
          onTokenRefreshFailed(new Error("Token refresh failed"));
        }
      }
      refreshSucceeded = false;
    }

    throw error;
  },
);

export default api;
