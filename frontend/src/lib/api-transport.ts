import axios from "axios";
import type { AxiosError, AxiosRequestConfig, AxiosResponse } from "axios";
import { getSession } from "next-auth/react";
import { env } from "~/env";
import { createLogger } from "~/lib/logger";

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
    // Only try to get session if we're in the browser
    if (globalThis.window !== undefined) {
      const session = await getSession();

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

// Helper: Set authorization header (handles both methods)
function setAuthorizationHeader(
  headers: AxiosRequestConfig["headers"],
  token: string,
): void {
  if (!headers) return;

  const headersObj = headers as Record<string, unknown> & {
    set?: (key: string, value: string) => void;
  };

  if (typeof headersObj.set === "function") {
    headersObj.set("Authorization", `Bearer ${token}`);
  } else {
    headersObj.Authorization = `Bearer ${token}`;
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

// Helper: Attempt server-side token refresh
async function attemptServerSideRefresh(
  originalRequest: AxiosRequestConfig,
): Promise<AxiosResponse | null> {
  try {
    const { refreshSessionTokensOnServer } =
      await import("~/server/auth/token-refresh");
    const refreshed = await refreshSessionTokensOnServer();

    if (!refreshed?.accessToken) {
      return null;
    }

    originalRequest.headers ??= {};
    setAuthorizationHeader(originalRequest.headers, refreshed.accessToken);
    onTokenRefreshed(refreshed.accessToken);
    return api(originalRequest);
  } catch {
    return null;
  }
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

  const session = await getSession();

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
        url: originalRequest?.url,
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
      url: originalRequest.url,
      retry_count: originalRequest._retryCount,
      caller_id: callerId,
    });

    // Limit retry attempts
    if (originalRequest._retryCount > 3) {
      logger.warn("max token refresh retries reached", {
        method: originalRequest.method?.toUpperCase(),
        url: originalRequest.url,
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
      // Server-side refresh
      if (globalThis.window === undefined) {
        logger.info("attempting server-side token refresh", {
          caller_id: callerId,
        });
        const result = await attemptServerSideRefresh(originalRequest);
        if (result) {
          logger.info("server-side token refresh successful", {
            caller_id: callerId,
          });
          return result;
        }
        logger.error("server-side token refresh failed", {
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
