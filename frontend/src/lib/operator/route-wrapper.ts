import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import {
  getOperatorToken,
  getOperatorRefreshToken,
  setOperatorTokens,
} from "./cookies";
import { handleApiError } from "../api-helpers";
import {
  type RouteContext,
  buildQueryString,
  extractParams,
  parseRequestBody,
  wrapInApiResponse,
  createUnauthorizedResponse,
} from "../route-wrapper-utils";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorRouteWrapper" });

// --- Singleflight dedup for concurrent refreshes (Fix 1) ---
let activeRefreshPromise: Promise<string | null> | null = null;
let refreshCache: { accessToken: string; expiresAt: number } | null = null;
const REFRESH_CACHE_TTL_MS = 10_000;

// --- Request queue for 401 handling (Fix 2) ---
let isRefreshing = false;
let lastRefreshedToken: string | null = null;
let refreshSubscribers: {
  resolve: (token: string) => void;
  reject: (error: Error) => void;
}[] = [];

function onTokenRefreshed(token: string): void {
  lastRefreshedToken = token;
  refreshSubscribers.forEach((subscriber) => subscriber.resolve(token));
  refreshSubscribers = [];
}

function onTokenRefreshFailed(error: Error): void {
  refreshSubscribers.forEach((subscriber) => subscriber.reject(error));
  refreshSubscribers = [];
}

/** @internal Reset module-level refresh state (test isolation only) */
export function _resetRefreshState(): void {
  activeRefreshPromise = null;
  refreshCache = null;
  isRefreshing = false;
  lastRefreshedToken = null;
  refreshSubscribers = [];
}

async function attemptTokenRefresh(): Promise<string | null> {
  // Check cache first — a recent refresh may have already completed
  if (refreshCache && Date.now() < refreshCache.expiresAt) {
    return refreshCache.accessToken;
  }

  // Join in-flight refresh if one exists
  if (activeRefreshPromise) {
    return activeRefreshPromise;
  }

  // Start new refresh and share via module-level promise
  activeRefreshPromise = (async (): Promise<string | null> => {
    try {
      const refreshToken = await getOperatorRefreshToken();
      if (!refreshToken) return null;

      const { getServerApiUrl } = await import("~/lib/server-api-url");
      const url = `${getServerApiUrl()}/operator/auth/refresh`;

      const response = await fetch(url, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${refreshToken}`,
          "Content-Type": "application/json",
        },
      });

      if (!response.ok) return null;

      const envelope = (await response.json()) as {
        data: { access_token: string; refresh_token: string };
      };
      await setOperatorTokens(
        envelope.data.access_token,
        envelope.data.refresh_token,
      );
      return envelope.data.access_token;
    } catch (error) {
      logger.warn("operator_transparent_refresh_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      return null;
    }
  })();

  try {
    const result = await activeRefreshPromise;
    // Cache result for late arrivals
    if (result) {
      refreshCache = {
        accessToken: result,
        expiresAt: Date.now() + REFRESH_CACHE_TTL_MS,
      };
    }
    return result;
  } finally {
    activeRefreshPromise = null;
  }
}

async function operatorServerFetch<T>(
  endpoint: string,
  token: string,
  options: { method: string; body?: unknown },
): Promise<T> {
  const { getServerApiUrl } = await import("~/lib/server-api-url");
  const url = `${getServerApiUrl()}${endpoint}`;

  const response = await fetch(url, {
    method: options.method,
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: options.body ? JSON.stringify(options.body) : undefined,
  });

  // On 401, attempt a transparent refresh and retry once
  if (response.status === 401) {
    // Queue request if refresh is already in progress
    if (isRefreshing) {
      return new Promise<T>((resolve, reject) => {
        refreshSubscribers.push({
          resolve: (newToken: string) => {
            fetch(url, {
              method: options.method,
              headers: {
                Authorization: `Bearer ${newToken}`,
                "Content-Type": "application/json",
              },
              body: options.body ? JSON.stringify(options.body) : undefined,
            })
              .then((retryResponse) => {
                if (!retryResponse.ok) {
                  retryResponse
                    .text()
                    .then((errorText) => {
                      reject(
                        new Error(
                          `API error (${retryResponse.status}): ${errorText}`,
                        ),
                      );
                    })
                    .catch(reject);
                  return;
                }
                resolve(parseResponse<T>(retryResponse));
              })
              .catch(reject);
          },
          reject: (error: Error) => {
            reject(error);
          },
        });
      });
    }

    isRefreshing = true;
    lastRefreshedToken = null;

    try {
      const newToken = await attemptTokenRefresh();
      if (newToken) {
        onTokenRefreshed(newToken);

        const retryResponse = await fetch(url, {
          method: options.method,
          headers: {
            Authorization: `Bearer ${newToken}`,
            "Content-Type": "application/json",
          },
          body: options.body ? JSON.stringify(options.body) : undefined,
        });

        if (!retryResponse.ok) {
          const errorText = await retryResponse.text();
          throw new Error(`API error (${retryResponse.status}): ${errorText}`);
        }

        return parseResponse<T>(retryResponse);
      }

      onTokenRefreshFailed(
        new Error(`API error (${response.status}): Unauthorized`),
      );
    } finally {
      isRefreshing = false;
      // Handle late arrivals queued after onTokenRefreshed but before finally
      if (refreshSubscribers.length > 0) {
        if (lastRefreshedToken) {
          onTokenRefreshed(lastRefreshedToken);
        } else {
          onTokenRefreshFailed(new Error("Token refresh failed"));
        }
      }
      lastRefreshedToken = null;
    }
  }

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`API error (${response.status}): ${errorText}`);
  }

  return parseResponse<T>(response);
}

function parseResponse<T>(response: Response): Promise<T> {
  if (response.status === 204) {
    return Promise.resolve(undefined as T);
  }

  return response.json().then((json: unknown) => {
    // Unwrap backend envelope { status, data, message } from common.Respond()
    if (
      typeof json === "object" &&
      json !== null &&
      "data" in json &&
      "status" in json
    ) {
      return (json as { data: T }).data;
    }
    return json as T;
  });
}

export function operatorApiGet<T>(endpoint: string, token: string): Promise<T> {
  return operatorServerFetch<T>(endpoint, token, { method: "GET" });
}

export function operatorApiPost<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return operatorServerFetch<T>(endpoint, token, { method: "POST", body });
}

export function operatorApiPut<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return operatorServerFetch<T>(endpoint, token, { method: "PUT", body });
}

export function operatorApiDelete<T>(
  endpoint: string,
  token: string,
): Promise<T> {
  return operatorServerFetch<T>(endpoint, token, { method: "DELETE" });
}

type NoBodyHandler<T> = (
  request: NextRequest,
  token: string,
  params: Record<string, unknown>,
) => Promise<T>;

type WithBodyHandler<T, B> = (
  request: NextRequest,
  body: B,
  token: string,
  params: Record<string, unknown>,
) => Promise<T>;

function createOperatorNoBodyHandler<T>(
  handler: NoBodyHandler<T>,
  formatResponse: (data: T) => NextResponse,
) {
  return async (
    request: NextRequest,
    context: RouteContext,
  ): Promise<NextResponse> => {
    try {
      const token = await getOperatorToken();
      if (!token) return createUnauthorizedResponse();
      const params = await extractParams(request, context);
      const data = await handler(request, token, params);
      return formatResponse(data);
    } catch (error) {
      return handleApiError(error);
    }
  };
}

function createOperatorWithBodyHandler<T, B>(handler: WithBodyHandler<T, B>) {
  return async (
    request: NextRequest,
    context: RouteContext,
  ): Promise<NextResponse> => {
    try {
      const token = await getOperatorToken();
      if (!token) return createUnauthorizedResponse();
      const params = await extractParams(request, context);
      const body = await parseRequestBody<B>(request);
      const data = await handler(request, body, token, params);
      return NextResponse.json(wrapInApiResponse(data));
    } catch (error) {
      return handleApiError(error);
    }
  };
}

const jsonResponse = <T>(data: T) => NextResponse.json(wrapInApiResponse(data));

export function createOperatorGetHandler<T>(handler: NoBodyHandler<T>) {
  return createOperatorNoBodyHandler(handler, jsonResponse);
}

export function createOperatorPostHandler<T, B = unknown>(
  handler: WithBodyHandler<T, B>,
) {
  return createOperatorWithBodyHandler(handler);
}

export function createOperatorPutHandler<T, B = unknown>(
  handler: WithBodyHandler<T, B>,
) {
  return createOperatorWithBodyHandler(handler);
}

export function createOperatorDeleteHandler<T>(handler: NoBodyHandler<T>) {
  return createOperatorNoBodyHandler(handler, (data: T) => {
    if (data === null || data === undefined) {
      return new NextResponse(null, { status: 204 });
    }
    return NextResponse.json(wrapInApiResponse(data));
  });
}

export function createOperatorProxyGetHandler<T>(backendEndpoint: string) {
  return createOperatorGetHandler<T>(
    async (request: NextRequest, token: string) => {
      const endpoint = `${backendEndpoint}${buildQueryString(request)}`;
      return operatorApiGet<T>(endpoint, token);
    },
  );
}

export { isStringParam } from "../route-wrapper-utils";
