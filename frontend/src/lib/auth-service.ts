// lib/auth-service.ts
/**
 * Auth Service for BetterAuth
 *
 * BetterAuth uses cookies for session management, so no JWT tokens are needed.
 * - Browser-side: cookies are sent automatically with credentials: "include"
 * - Server-side: cookies are forwarded via Cookie header
 *
 * Note: Many functions in this file are for legacy NextAuth-based admin operations.
 * The primary authentication flow now uses the auth-client module.
 */
import { isAxiosError } from "axios";
import { env } from "~/env";
import api from "./api";
import {
  mapAccountResponse,
  mapRoleResponse,
  mapPermissionResponse,
  mapTokenResponse,
  mapParentAccountResponse,
  type Account,
  type Role,
  type Permission,
  type Token,
  type ParentAccount,
  type LoginRequest,
  type RegisterRequest,
  type TokenResponse,
  type ChangePasswordRequest,
  type PasswordResetRequest,
  type PasswordResetConfirmRequest,
  type CreateRoleRequest,
  type UpdateRoleRequest,
  type CreatePermissionRequest,
  type UpdatePermissionRequest,
  type UpdateAccountRequest,
  type CreateParentAccountRequest,
  type BackendAccount,
  type BackendRole,
  type BackendPermission,
  type BackendToken,
  type BackendParentAccount,
} from "./auth-helpers";
import type { AxiosError } from "axios";
import type { ApiError } from "./auth-api";

// Generic API response interface
interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

// Specific response types
interface TokenCleanupResponse {
  cleaned_tokens: number;
}

// Interface for raw API responses that use lowercase field names
interface RawRoleData {
  id?: number;
  name?: string;
  description?: string;
  created_at?: string;
  createdAt?: string;
  updated_at?: string;
  updatedAt?: string;
  permissions?: BackendPermission[];
}

/**
 * Extract BackendRole from various nested response formats.
 * Handles: direct BackendRole, { data: BackendRole }, { data: { data: BackendRole } }
 */
function extractBackendRole(responseData: unknown): BackendRole {
  if (!responseData || typeof responseData !== "object") {
    throw new Error("Invalid response format from role API");
  }

  const data = responseData as Record<string, unknown>;

  // Direct BackendRole (has ID and Name)
  if ("ID" in data && "Name" in data) {
    return data as unknown as BackendRole;
  }

  // Direct BackendRole with lowercase (has id and name)
  if ("id" in data && "name" in data && !("data" in data)) {
    return data as unknown as BackendRole;
  }

  // Nested: { data: ... }
  if ("data" in data && data.data) {
    const nested = data.data as Record<string, unknown>;

    // Double nested: { data: { data: BackendRole } }
    if (typeof nested === "object" && "data" in nested && nested.data) {
      return nested.data as BackendRole;
    }

    // Single nested: { data: BackendRole }
    return nested as unknown as BackendRole;
  }

  console.error("Unexpected role response structure:", responseData);
  throw new Error("Invalid response format from role API");
}

/**
 * Normalize role data casing from lowercase API response to uppercase BackendRole.
 */
function normalizeRoleCasing(roleData: BackendRole): BackendRole {
  // Already has uppercase fields
  if ("ID" in roleData) {
    return roleData;
  }

  // Convert lowercase to uppercase
  const raw = roleData as unknown as RawRoleData;
  if (raw.id !== undefined) {
    return {
      ID: raw.id,
      Name: raw.name ?? "",
      Description: raw.description ?? "",
      CreatedAt: raw.created_at ?? raw.createdAt ?? "",
      UpdatedAt: raw.updated_at ?? raw.updatedAt ?? "",
      Permissions: raw.permissions,
    };
  }

  return roleData;
}

interface PasswordResetResponse {
  message: string;
}

interface ApiErrorResponseBody {
  error?: string;
  message?: string;
}

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
  let message = fallbackMessage;

  try {
    const contentType = response.headers.get("Content-Type") ?? "";
    if (contentType.includes("application/json")) {
      const payload = (await response.json()) as ApiErrorResponseBody;
      message = payload.error ?? payload.message ?? fallbackMessage;
    } else {
      const text = (await response.text()).trim();
      if (text) {
        message = text;
      }
    }
  } catch (parseError) {
    console.warn("Failed to parse password reset error response", parseError);
  }

  const apiError = new Error(message) as ApiError;
  apiError.status = response.status;

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
  let message = fallbackMessage;

  const data = error.response?.data;
  if (data) {
    if (typeof data === "string") {
      message = data;
    } else {
      message = data.error ?? data.message ?? fallbackMessage;
    }
  } else if (error.message) {
    message = error.message;
  }

  const apiError = new Error(message) as ApiError;
  apiError.status = error.response?.status;

  const headers = error.response?.headers as
    | Record<string, unknown>
    | undefined;
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
// Generic Auth API Helper - Eliminates ~300 lines of duplication
// ============================================================================

type HttpMethod = "GET" | "POST" | "PUT" | "DELETE";

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
 * BetterAuth uses cookies, so no Authorization header is needed.
 * Cookies are sent automatically with credentials: "include".
 */
function buildAuthHeaders(_requiresAuth: boolean): Record<string, string> {
  return {
    "Content-Type": "application/json",
  };
}

/**
 * Execute fetch request in browser context.
 * Returns null for 204/empty responses (void endpoints).
 * BetterAuth uses cookies, sent automatically with credentials: "include".
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
    credentials: "include", // Send cookies automatically
    ...(body !== undefined && { body: JSON.stringify(body) }),
  });

  if (!response.ok) {
    const errorText = await response.text();
    console.error(`${errorPrefix} error: ${response.status}`, errorText);
    throw new Error(`${errorPrefix} failed: ${response.status}`);
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
    case "POST":
      return (await api.post<ApiResponse<TBackend>>(url, body, config)).data;
    case "PUT":
      return (await api.put<ApiResponse<TBackend>>(url, body, config)).data;
    case "DELETE":
      return (await api.delete<ApiResponse<TBackend>>(url, config)).data;
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
  const baseUrl = useProxyApi ? "/api" : env.NEXT_PUBLIC_API_URL;
  const url = `${baseUrl}${endpoint}`;

  try {
    let backendData: TBackend;

    if (useProxyApi) {
      const headers = buildAuthHeaders(requiresAuth);
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
    console.error(`${errorPrefix} error:`, error);
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
 * Helper for void responses (POST/PUT/DELETE that don't return data).
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
  console.error("Unexpected permissions response structure:", response);
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
  console.error("Unexpected roles response structure:", response);
  throw new Error("Invalid response format from roles API");
}

export const authService = {
  // Public endpoints
  // Note: For BetterAuth, use signIn.email() from auth-client instead
  // This is kept for backward compatibility with legacy admin flows
  login: async (credentials: LoginRequest): Promise<TokenResponse> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? "/api/auth/login"
      : `${env.NEXT_PUBLIC_API_URL}/auth/login`;

    try {
      if (useProxyApi) {
        const response = await fetch(url, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          credentials: "include", // Send cookies automatically
          body: JSON.stringify(credentials),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`Login error: ${response.status}`, errorText);
          throw new Error(`Login failed: ${response.status}`);
        }

        return (await response.json()) as TokenResponse;
      } else {
        const response = await api.post<TokenResponse>(url, credentials);
        return response.data;
      }
    } catch (error) {
      console.error("Login error:", error);
      throw error;
    }
  },

  register: async (data: RegisterRequest): Promise<Account> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? "/api/auth/register"
      : `${env.NEXT_PUBLIC_API_URL}/auth/register`;

    try {
      if (useProxyApi) {
        const response = await fetch(url, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          credentials: "include", // Send cookies automatically
          body: JSON.stringify({
            email: data.email,
            username: data.username,
            name: data.name,
            password: data.password,
            confirm_password: data.confirmPassword,
          }),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`Registration error: ${response.status}`, errorText);
          throw new Error(`Registration failed: ${response.status}`);
        }

        const responseData =
          (await response.json()) as ApiResponse<BackendAccount>;
        return mapAccountResponse(responseData.data);
      } else {
        const response = await api.post<ApiResponse<BackendAccount>>(url, {
          email: data.email,
          username: data.username,
          name: data.name,
          password: data.password,
          confirm_password: data.confirmPassword,
        });
        return mapAccountResponse(response.data.data);
      }
    } catch (error) {
      console.error("Registration error:", error);
      throw error;
    }
  },

  // Note: For BetterAuth, use signOut() from auth-client instead
  // This is kept for backward compatibility with legacy admin flows
  logout: async (): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? "/api/auth/logout"
      : `${env.NEXT_PUBLIC_API_URL}/auth/logout`;

    try {
      if (useProxyApi) {
        // BetterAuth uses cookies, sent automatically with credentials: "include"
        const response = await fetch(url, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          credentials: "include", // Send cookies automatically
        });

        if (!response.ok && response.status !== 204) {
          console.error(`Logout error: ${response.status}`);
        }
      } else {
        await api.post(url);
      }
    } catch (error) {
      console.error("Logout error:", error);
      // Don't throw - logout should always succeed on the client side
    }
  },

  // Note: BetterAuth handles token refresh automatically via cookies
  // This is kept for backward compatibility with legacy admin flows
  refreshToken: async (_refreshToken: string): Promise<TokenResponse> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? "/api/auth/refresh"
      : `${env.NEXT_PUBLIC_API_URL}/auth/refresh`;

    try {
      if (useProxyApi) {
        // BetterAuth uses cookies for session management
        const response = await fetch(url, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          credentials: "include", // Send cookies automatically
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`Token refresh error: ${response.status}`, errorText);
          throw new Error(`Token refresh failed: ${response.status}`);
        }

        return (await response.json()) as TokenResponse;
      } else {
        const response = await api.post<TokenResponse>(url, {});
        return response.data;
      }
    } catch (error) {
      console.error("Token refresh error:", error);
      throw error;
    }
  },

  initiatePasswordReset: async (data: PasswordResetRequest): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? "/api/auth/password-reset"
      : `${env.NEXT_PUBLIC_API_URL}/auth/password-reset`;

    try {
      if (useProxyApi) {
        const response = await fetch(url, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          credentials: "include", // Send cookies if present
          body: JSON.stringify(data),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`Password reset error: ${response.status}`, errorText);
          throw new Error(`Password reset failed: ${response.status}`);
        }
      } else {
        await api.post(url, data);
      }
    } catch (error) {
      console.error("Password reset error:", error);
      throw error;
    }
  },

  resetPassword: async (
    data: PasswordResetConfirmRequest,
  ): Promise<PasswordResetResponse> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? "/api/auth/password-reset/confirm"
      : `${env.NEXT_PUBLIC_API_URL}/auth/password-reset/confirm`;
    const fallbackMessage = "Fehler beim Zurücksetzen des Passworts";
    const payload = {
      token: data.token,
      new_password: data.newPassword,
      confirm_password: data.confirmPassword,
    };

    try {
      if (useProxyApi) {
        const response = await fetch(url, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          credentials: "include", // Send cookies if present
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

      console.error("Password reset confirm error:", error);
      const fallbackError = new Error(fallbackMessage) as ApiError;
      throw fallbackError;
    }
  },

  // Protected endpoints (require authentication)
  getAccount: async (): Promise<Account> =>
    authFetch<BackendAccount, Account>("/auth/account", {
      mapper: mapAccountResponse,
      errorPrefix: "Get account",
    }),

  changePassword: async (data: ChangePasswordRequest): Promise<void> =>
    authFetchVoid("/auth/password", {
      method: "POST",
      body: {
        current_password: data.currentPassword,
        new_password: data.newPassword,
        confirm_password: data.confirmPassword,
      },
      errorPrefix: "Change password",
    }),

  // Admin endpoints - Role management
  createRole: async (data: CreateRoleRequest): Promise<Role> =>
    authFetch<BackendRole, Role>("/auth/roles", {
      method: "POST",
      body: data,
      mapper: mapRoleResponse,
      errorPrefix: "Create role",
    }),

  getRoles: async (filters?: { name?: string }): Promise<Role[]> => {
    const params = new URLSearchParams();
    if (filters?.name) params.append("name", filters.name);
    const queryString = params.toString();
    const endpoint = queryString ? `/auth/roles?${queryString}` : "/auth/roles";
    return authFetchList<BackendRole, Role>(endpoint, mapRoleResponse, {
      errorPrefix: "Get roles",
    });
  },

  getRole: async (id: string): Promise<Role> =>
    authFetch<BackendRole, Role>(`/auth/roles/${id}`, {
      mapper: (data) => mapRoleResponse(normalizeRoleCasing(data)),
      extractData: extractBackendRole,
      errorPrefix: "Get role",
    }),

  updateRole: async (id: string, data: UpdateRoleRequest): Promise<void> =>
    authFetchVoid(`/auth/roles/${id}`, {
      method: "PUT",
      body: data,
      errorPrefix: "Update role",
    }),

  deleteRole: async (id: string): Promise<void> =>
    authFetchVoid(`/auth/roles/${id}`, {
      method: "DELETE",
      errorPrefix: "Delete role",
    }),

  getRolePermissions: async (roleId: string): Promise<Permission[]> =>
    authFetchList<BackendPermission, Permission>(
      `/auth/roles/${roleId}/permissions`,
      mapPermissionResponse,
      {
        extractData: extractNestedPermissions,
        errorPrefix: "Get role permissions",
      },
    ),

  assignPermissionToRole: async (
    roleId: string,
    permissionId: string,
  ): Promise<void> =>
    authFetchVoid(`/auth/roles/${roleId}/permissions/${permissionId}`, {
      method: "POST",
      errorPrefix: "Assign permission to role",
    }),

  removePermissionFromRole: async (
    roleId: string,
    permissionId: string,
  ): Promise<void> =>
    authFetchVoid(`/auth/roles/${roleId}/permissions/${permissionId}`, {
      method: "DELETE",
      errorPrefix: "Remove permission from role",
    }),

  // Admin endpoints - Permission management
  createPermission: async (
    data: CreatePermissionRequest,
  ): Promise<Permission> =>
    authFetch<BackendPermission, Permission>("/auth/permissions", {
      method: "POST",
      body: data,
      mapper: mapPermissionResponse,
      errorPrefix: "Create permission",
    }),

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

  getPermission: async (id: string): Promise<Permission> =>
    authFetch<BackendPermission, Permission>(`/auth/permissions/${id}`, {
      mapper: mapPermissionResponse,
      errorPrefix: "Get permission",
    }),

  updatePermission: async (
    id: string,
    data: UpdatePermissionRequest,
  ): Promise<void> =>
    authFetchVoid(`/auth/permissions/${id}`, {
      method: "PUT",
      body: data,
      errorPrefix: "Update permission",
    }),

  deletePermission: async (id: string): Promise<void> =>
    authFetchVoid(`/auth/permissions/${id}`, {
      method: "DELETE",
      errorPrefix: "Delete permission",
    }),

  // Admin endpoints - Account management
  getAccounts: async (filters?: {
    email?: string;
    active?: boolean;
  }): Promise<Account[]> => {
    const params = new URLSearchParams();
    if (filters?.email) params.append("email", filters.email);
    if (filters?.active !== undefined)
      params.append("active", filters.active.toString());
    const queryString = params.toString();
    const endpoint = queryString
      ? `/auth/accounts?${queryString}`
      : "/auth/accounts";
    return authFetchList<BackendAccount, Account>(
      endpoint,
      mapAccountResponse,
      {
        errorPrefix: "Get accounts",
      },
    );
  },

  getAccountById: async (id: string): Promise<Account> =>
    authFetch<BackendAccount, Account>(`/auth/accounts/${id}`, {
      mapper: mapAccountResponse,
      errorPrefix: "Get account",
    }),

  updateAccount: async (
    id: string,
    data: UpdateAccountRequest,
  ): Promise<void> =>
    authFetchVoid(`/auth/accounts/${id}`, {
      method: "PUT",
      body: data,
      errorPrefix: "Update account",
    }),

  activateAccount: async (id: string): Promise<void> =>
    authFetchVoid(`/auth/accounts/${id}/activate`, {
      method: "PUT",
      errorPrefix: "Activate account",
    }),

  deactivateAccount: async (id: string): Promise<void> =>
    authFetchVoid(`/auth/accounts/${id}/deactivate`, {
      method: "PUT",
      errorPrefix: "Deactivate account",
    }),

  getAccountsByRole: async (roleName: string): Promise<Account[]> =>
    authFetchList<BackendAccount, Account>(
      `/auth/accounts/by-role/${roleName}`,
      mapAccountResponse,
      { errorPrefix: "Get accounts by role" },
    ),

  assignRoleToAccount: async (
    accountId: string,
    roleId: string,
  ): Promise<void> =>
    authFetchVoid(`/auth/accounts/${accountId}/roles/${roleId}`, {
      method: "POST",
      errorPrefix: "Assign role to account",
    }),

  removeRoleFromAccount: async (
    accountId: string,
    roleId: string,
  ): Promise<void> =>
    authFetchVoid(`/auth/accounts/${accountId}/roles/${roleId}`, {
      method: "DELETE",
      errorPrefix: "Remove role from account",
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

  getAccountPermissions: async (accountId: string): Promise<Permission[]> =>
    authFetchList<BackendPermission, Permission>(
      `/auth/accounts/${accountId}/permissions`,
      mapPermissionResponse,
      {
        extractData: extractNestedPermissions,
        errorPrefix: "Get account permissions",
      },
    ),

  getAccountDirectPermissions: async (
    accountId: string,
  ): Promise<Permission[]> =>
    authFetchList<BackendPermission, Permission>(
      `/auth/accounts/${accountId}/permissions/direct`,
      mapPermissionResponse,
      {
        extractData: extractNestedPermissions,
        errorPrefix: "Get account direct permissions",
      },
    ),

  grantPermissionToAccount: async (
    accountId: string,
    permissionId: string,
  ): Promise<void> =>
    authFetchVoid(
      `/auth/accounts/${accountId}/permissions/${permissionId}/grant`,
      {
        method: "POST",
        errorPrefix: "Grant permission to account",
      },
    ),

  denyPermissionToAccount: async (
    accountId: string,
    permissionId: string,
  ): Promise<void> =>
    authFetchVoid(
      `/auth/accounts/${accountId}/permissions/${permissionId}/deny`,
      {
        method: "POST",
        errorPrefix: "Deny permission to account",
      },
    ),

  removePermissionFromAccount: async (
    accountId: string,
    permissionId: string,
  ): Promise<void> =>
    authFetchVoid(`/auth/accounts/${accountId}/permissions/${permissionId}`, {
      method: "DELETE",
      errorPrefix: "Remove permission from account",
    }),

  assignPermissionToAccount: async (
    accountId: string,
    permissionId: string,
  ): Promise<void> =>
    // Use the grant endpoint for assigning permissions
    authService.grantPermissionToAccount(accountId, permissionId),

  // Get all available permissions for assignment
  getAvailablePermissions: async (): Promise<Permission[]> =>
    authFetchList<BackendPermission, Permission>(
      "/auth/permissions",
      mapPermissionResponse,
      { errorPrefix: "Get available permissions" },
    ),

  // Admin endpoints - Token management
  getActiveTokens: async (accountId: string): Promise<Token[]> =>
    authFetchList<BackendToken, Token>(
      `/auth/accounts/${accountId}/tokens`,
      mapTokenResponse,
      { errorPrefix: "Get active tokens" },
    ),

  revokeAllTokens: async (accountId: string): Promise<void> =>
    authFetchVoid(`/auth/accounts/${accountId}/tokens`, {
      method: "DELETE",
      errorPrefix: "Revoke all tokens",
    }),

  cleanupExpiredTokens: async (): Promise<number> =>
    authFetch<TokenCleanupResponse, number>("/auth/tokens/expired", {
      method: "DELETE",
      mapper: (data) => data.cleaned_tokens,
      errorPrefix: "Cleanup expired tokens",
    }),

  // Admin endpoints - Parent account management
  createParentAccount: async (
    data: CreateParentAccountRequest,
  ): Promise<ParentAccount> =>
    authFetch<BackendParentAccount, ParentAccount>("/auth/parent-accounts", {
      method: "POST",
      body: {
        email: data.email,
        username: data.username,
        password: data.password,
        confirm_password: data.confirmPassword,
      },
      mapper: mapParentAccountResponse,
      errorPrefix: "Create parent account",
    }),

  getParentAccounts: async (filters?: {
    email?: string;
    active?: boolean;
  }): Promise<ParentAccount[]> => {
    const params = new URLSearchParams();
    if (filters?.email) params.append("email", filters.email);
    if (filters?.active !== undefined)
      params.append("active", filters.active.toString());
    const queryString = params.toString();
    const endpoint = queryString
      ? `/auth/parent-accounts?${queryString}`
      : "/auth/parent-accounts";
    return authFetchList<BackendParentAccount, ParentAccount>(
      endpoint,
      mapParentAccountResponse,
      { errorPrefix: "Get parent accounts" },
    );
  },

  getParentAccount: async (id: string): Promise<ParentAccount> =>
    authFetch<BackendParentAccount, ParentAccount>(
      `/auth/parent-accounts/${id}`,
      {
        mapper: mapParentAccountResponse,
        errorPrefix: "Get parent account",
      },
    ),

  updateParentAccount: async (
    id: string,
    data: { email: string; username?: string },
  ): Promise<void> =>
    authFetchVoid(`/auth/parent-accounts/${id}`, {
      method: "PUT",
      body: data,
      errorPrefix: "Update parent account",
    }),

  activateParentAccount: async (id: string): Promise<void> =>
    authFetchVoid(`/auth/parent-accounts/${id}/activate`, {
      method: "PUT",
      errorPrefix: "Activate parent account",
    }),

  deactivateParentAccount: async (id: string): Promise<void> =>
    authFetchVoid(`/auth/parent-accounts/${id}/deactivate`, {
      method: "PUT",
      errorPrefix: "Deactivate parent account",
    }),
};
