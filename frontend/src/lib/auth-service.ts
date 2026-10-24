// lib/auth-service.ts
import { getCachedSession } from "./session-cache";
import { isAxiosError } from "axios";
import api from "./api-transport";
import { resolveApiUrl } from "./api-url";
import { createLogger } from "~/lib/logger";

// Logger instance for auth service
const logger = createLogger({ component: "AuthService" });
import {
  mapRoleResponse,
  mapPermissionResponse,
  type Role,
  type Permission,
  type PasswordResetConfirmRequest,
  type BackendRole,
  type BackendPermission,
} from "./auth-helpers";
import type { AxiosError } from "axios";
import {
  type ApiError,
  apiErrorFromBody,
  apiErrorFromText,
  transportFetch,
  unavailableApiError,
} from "./api-error";

// Generic API response interface
interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

interface PasswordResetResponse {
  message: string;
}

type ApiErrorResponseBody = Record<string, unknown>;

function parseRetryAfter(value: string | null | undefined): number | null {
  if (!value) {
    return null;
  }

  const numeric = Number(value);
  if (!Number.isNaN(numeric)) {
    return Math.max(0, Math.round(numeric));
  }

  const date = Date.parse(value);
  if (!Number.isNaN(date)) {
    const diffMs = date - Date.now();
    return diffMs > 0 ? Math.ceil(diffMs / 1000) : 0;
  }

  return null;
}

async function buildFetchApiError(
  response: Response,
  fallbackMessage: string,
): Promise<ApiError> {
  let text = "";

  try {
    text = await response.text();
  } catch (readError) {
    logger.warn("failed to parse password reset error response", {
      error: readError instanceof Error ? readError.message : String(readError),
    });
  }

  // A real ApiError with code, field errors and request ID, so the shared
  // error path can show the catalog text (#2517). The envelope's `detail`
  // becomes the diagnostic message.
  const apiError = apiErrorFromText(fallbackMessage, response.status, text);

  const retryAfterSeconds = parseRetryAfter(
    response.headers.get("Retry-After"),
  );
  if (retryAfterSeconds !== null) {
    apiError.retryAfterSeconds = retryAfterSeconds;
  }

  return apiError;
}

function buildAxiosApiError(
  error: AxiosError<ApiErrorResponseBody>,
  fallbackMessage: string,
): ApiError {
  if (!error.response) {
    return unavailableApiError(error);
  }

  let message = fallbackMessage;

  const data = error.response.data;
  if (data) {
    if (typeof data === "string") {
      message = data;
    }
  } else if (error.message) {
    message = error.message;
  }

  const apiError = apiErrorFromBody(
    message,
    error.response.status,
    typeof data === "object" ? data : undefined,
  );

  const headers = error.response.headers as Record<string, unknown> | undefined;
  const retryAfterHeader = headers ? headers["retry-after"] : undefined;
  let retryAfterValue: string | null = null;
  if (Array.isArray(retryAfterHeader)) {
    const firstString = retryAfterHeader.find(
      (value): value is string => typeof value === "string",
    );
    if (firstString) {
      retryAfterValue = firstString;
    }
  } else if (typeof retryAfterHeader === "string") {
    retryAfterValue = retryAfterHeader;
  }

  const retryAfterSeconds = parseRetryAfter(retryAfterValue);
  if (retryAfterSeconds !== null) {
    apiError.retryAfterSeconds = retryAfterSeconds;
  }

  return apiError;
}

// ============================================================================
// Generic Auth API Helper
// ============================================================================

// The remaining callers only read (GET) and replace (PUT).
type HttpMethod = "GET" | "PUT";

interface AuthFetchOptions<TBackend, TFrontend> {
  /** HTTP method (default: GET) */
  method?: HttpMethod;
  /** Request body (will be JSON stringified) */
  body?: unknown;
  /** Map backend response to frontend type. If omitted, returns void. */
  mapper?: (data: TBackend) => TFrontend;
  /** Whether this endpoint requires authentication (default: true) */
  requiresAuth?: boolean;
  /** Custom error message prefix for logging */
  errorPrefix?: string;
  /** Extract data from nested response structure (default: extract from .data) */
  extractData?: (response: unknown) => TBackend;
}

/**
 * Build auth headers for browser-side fetch requests.
 */
async function buildAuthHeaders(
  requiresAuth: boolean,
): Promise<Record<string, string>> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };

  if (!requiresAuth) {
    return headers;
  }

  const session = await getCachedSession();
  if (session?.user?.token) {
    headers.Authorization = `Bearer ${session.user.token}`;
  }

  return headers;
}

/**
 * Execute fetch request in browser context.
 * Returns null for 204/empty responses (void endpoints).
 */
async function executeBrowserFetch<TBackend>(
  url: string,
  method: HttpMethod,
  headers: Record<string, string>,
  body: unknown,
  errorPrefix: string,
): Promise<TBackend | null> {
  const response = await fetch(url, {
    method,
    headers,
    ...(body !== undefined && { body: JSON.stringify(body) }),
  });

  if (!response.ok) {
    const errorText = await response.text();
    logger.error("api error during auth operation", {
      operation: errorPrefix,
      status: response.status,
      error_text: errorText.substring(0, 200),
    });
    throw apiErrorFromText(
      `${errorPrefix} failed: ${response.status}`,
      response.status,
      errorText,
    );
  }

  // Handle 204 No Content and empty responses (void endpoints)
  if (response.status === 204) {
    return null;
  }

  const text = await response.text();
  if (!text) {
    return null;
  }

  return JSON.parse(text) as TBackend;
}

/**
 * Execute axios request in server context.
 */
async function executeServerFetch<TBackend>(
  url: string,
  method: HttpMethod,
  body: unknown,
  requiresAuth: boolean,
): Promise<ApiResponse<TBackend>> {
  const config = requiresAuth ? undefined : {};

  switch (method) {
    case "GET":
      return (await api.get<ApiResponse<TBackend>>(url, config)).data;
    case "PUT":
      return (await api.put<ApiResponse<TBackend>>(url, body, config)).data;
  }
}

/**
 * Generic helper for auth service API calls.
 * Handles proxy vs direct API calls, authentication, error handling, and response mapping.
 */
async function authFetch<TBackend, TFrontend = void>(
  endpoint: string,
  options: AuthFetchOptions<TBackend, TFrontend> = {},
): Promise<TFrontend> {
  const {
    method = "GET",
    body,
    mapper,
    requiresAuth = true,
    errorPrefix = endpoint,
    extractData,
  } = options;

  const useProxyApi = globalThis.window !== undefined;
  const url = await resolveApiUrl(`/api${endpoint}`, endpoint);

  try {
    let backendData: TBackend;

    if (useProxyApi) {
      const headers = await buildAuthHeaders(requiresAuth);
      const responseData = await executeBrowserFetch<unknown>(
        url,
        method,
        headers,
        body,
        errorPrefix,
      );

      // Handle void responses (204/empty body)
      if (responseData === null) {
        return undefined as TFrontend;
      }

      backendData = extractData
        ? extractData(responseData)
        : (responseData as ApiResponse<TBackend>).data;
    } else {
      const responseData = await executeServerFetch<TBackend>(
        url,
        method,
        body,
        requiresAuth,
      );

      // Apply extractData on server path (matches browser behavior)
      backendData = extractData ? extractData(responseData) : responseData.data;
    }

    return mapper ? mapper(backendData) : (undefined as TFrontend);
  } catch (error) {
    logger.error("auth operation error", {
      operation: errorPrefix,
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }
}

/**
 * Helper for array responses that need mapping.
 */
async function authFetchList<TBackend, TFrontend>(
  endpoint: string,
  mapper: (data: TBackend) => TFrontend,
  options: Omit<AuthFetchOptions<TBackend[], TFrontend[]>, "mapper"> = {},
): Promise<TFrontend[]> {
  return authFetch<TBackend[], TFrontend[]>(endpoint, {
    ...options,
    mapper: (data) => data.map(mapper),
  });
}

/**
 * Helper for void responses (PUT requests that don't return data).
 */
async function authFetchVoid(
  endpoint: string,
  options: Omit<AuthFetchOptions<unknown, void>, "mapper"> = {},
): Promise<void> {
  await authFetch<unknown, void>(endpoint, options);
}

/**
 * Extract permissions from nested response structures.
 * Handles: { data: [] }, { data: { data: [] } }
 */
function extractNestedPermissions(response: unknown): BackendPermission[] {
  const data = response as {
    data?: { data?: BackendPermission[] } | BackendPermission[];
  };

  if (
    data?.data &&
    typeof data.data === "object" &&
    "data" in data.data &&
    Array.isArray(data.data.data)
  ) {
    return data.data.data;
  }
  if (data?.data && Array.isArray(data.data)) {
    return data.data;
  }
  logger.error("unexpected permissions response structure", {
    response_type: typeof response,
  });
  throw new Error("Invalid response format from permissions API");
}

/**
 * Extract roles from nested response structures.
 * Handles: [], { data: [] }, { data: { data: [] } }
 */
function extractNestedRoles(response: unknown): BackendRole[] {
  if (Array.isArray(response)) {
    return response as BackendRole[];
  }

  const data = response as {
    data?: { data?: BackendRole[] } | BackendRole[];
  };

  if (
    data?.data &&
    typeof data.data === "object" &&
    "data" in data.data &&
    Array.isArray(data.data.data)
  ) {
    return data.data.data;
  }
  if (data?.data && Array.isArray(data.data)) {
    return data.data;
  }
  logger.error("unexpected roles response structure", {
    response_type: typeof response,
  });
  throw new Error("Invalid response format from roles API");
}

export const authService = {
  resetPassword: async (
    data: PasswordResetConfirmRequest,
  ): Promise<PasswordResetResponse> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = await resolveApiUrl(
      "/api/auth/password-reset/confirm",
      "/auth/password-reset/confirm",
    );
    const fallbackMessage = "Fehler beim Zurücksetzen des Passworts";
    const payload = {
      token: data.token,
      new_password: data.newPassword,
      confirm_password: data.confirmPassword,
    };

    try {
      if (useProxyApi) {
        const response = await transportFetch(url, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        });

        if (!response.ok) {
          throw await buildFetchApiError(response, fallbackMessage);
        }

        try {
          return (await response.json()) as PasswordResetResponse;
        } catch {
          return { message: "Password reset successfully" };
        }
      }

      const response = await api.post<PasswordResetResponse>(url, payload);
      if (response.data && typeof response.data.message === "string") {
        return response.data;
      }

      return { message: "Password reset successfully" };
    } catch (error) {
      const apiError = error as ApiError | undefined;

      if (apiError?.status !== undefined) {
        throw apiError;
      }

      if (globalThis.window === undefined && isAxiosError(error)) {
        throw buildAxiosApiError(
          error as AxiosError<ApiErrorResponseBody>,
          fallbackMessage,
        );
      }

      logger.error("password reset confirm error", {
        error: error instanceof Error ? error.message : String(error),
      });
      const fallbackError = new Error(fallbackMessage) as ApiError;
      throw fallbackError;
    }
  },

  // Admin endpoints - Role management
  getRoles: async (filters?: { name?: string }): Promise<Role[]> => {
    const params = new URLSearchParams();
    if (filters?.name) params.append("name", filters.name);
    const queryString = params.toString();
    const endpoint = queryString ? `/auth/roles?${queryString}` : "/auth/roles";
    return authFetchList<BackendRole, Role>(endpoint, mapRoleResponse, {
      errorPrefix: "Get roles",
    });
  },

  getRolePermissions: async (roleId: string): Promise<Permission[]> =>
    authFetchList<BackendPermission, Permission>(
      `/auth/roles/${roleId}/permissions`,
      mapPermissionResponse,
      {
        extractData: extractNestedPermissions,
        errorPrefix: "Get role permissions",
      },
    ),

  replaceRolePermissions: async (
    roleId: string,
    permissionIds: readonly string[],
  ): Promise<void> =>
    authFetchVoid(`/auth/roles/${roleId}/permissions`, {
      method: "PUT",
      body: { permission_ids: permissionIds },
      errorPrefix: "Replace role permissions",
    }),

  // Admin endpoints - Permission management
  getPermissions: async (filters?: {
    resource?: string;
    action?: string;
  }): Promise<Permission[]> => {
    const params = new URLSearchParams();
    if (filters?.resource) params.append("resource", filters.resource);
    if (filters?.action) params.append("action", filters.action);
    const queryString = params.toString();
    const endpoint = queryString
      ? `/auth/permissions?${queryString}`
      : "/auth/permissions";
    return authFetchList<BackendPermission, Permission>(
      endpoint,
      mapPermissionResponse,
      { errorPrefix: "Get permissions" },
    );
  },

  // Admin endpoints - Account role assignment
  /**
   * Macht `roleId` zur einzigen Systemrolle des Kontos an dieser Schule.
   * Andere Systemrollen entfernt das Backend; Elternzugang (guardian) bleibt.
   */
  replaceAccountRole: async (
    accountId: string,
    roleId: string,
  ): Promise<void> =>
    authFetchVoid(`/auth/accounts/${accountId}/roles`, {
      method: "PUT",
      body: { role_id: roleId },
      errorPrefix: "Replace account role",
    }),

  getAccountRoles: async (accountId: string): Promise<Role[]> =>
    authFetchList<BackendRole, Role>(
      `/auth/accounts/${accountId}/roles`,
      mapRoleResponse,
      {
        extractData: extractNestedRoles,
        errorPrefix: "Get account roles",
      },
    ),
};
