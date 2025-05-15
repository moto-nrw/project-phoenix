// lib/auth-service.ts
import { getSession } from "next-auth/react";
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

export const authService = {
    // Public endpoints
    login: async (credentials: LoginRequest): Promise<TokenResponse> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/auth/login"
            : `${env.NEXT_PUBLIC_API_URL}/auth/login`;

        try {
            if (useProxyApi) {
                const response = await fetch(url, {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify(credentials),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Login error: ${response.status}`, errorText);
                    throw new Error(`Login failed: ${response.status}`);
                }

                return await response.json() as TokenResponse;
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
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/auth/register"
            : `${env.NEXT_PUBLIC_API_URL}/auth/register`;

        try {
            if (useProxyApi) {
                const response = await fetch(url, {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
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

                const responseData = await response.json() as ApiResponse<BackendAccount>;
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

    logout: async (): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/auth/logout"
            : `${env.NEXT_PUBLIC_API_URL}/auth/logout`;

        try {
            const session = await getSession();
            if (!session?.user?.token) {
                return; // Already logged out
            }

            if (useProxyApi) {
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session.user.token}`,
                        "Content-Type": "application/json",
                    },
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

    refreshToken: async (refreshToken: string): Promise<TokenResponse> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/auth/refresh"
            : `${env.NEXT_PUBLIC_API_URL}/auth/refresh`;

        try {
            if (useProxyApi) {
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${refreshToken}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Token refresh error: ${response.status}`, errorText);
                    throw new Error(`Token refresh failed: ${response.status}`);
                }

                return await response.json() as TokenResponse;
            } else {
                const response = await api.post<TokenResponse>(url, {}, {
                    headers: { Authorization: `Bearer ${refreshToken}` },
                });
                return response.data;
            }
        } catch (error) {
            console.error("Token refresh error:", error);
            throw error;
        }
    },

    initiatePasswordReset: async (data: PasswordResetRequest): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/auth/password-reset"
            : `${env.NEXT_PUBLIC_API_URL}/auth/password-reset`;

        try {
            if (useProxyApi) {
                const response = await fetch(url, {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
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

    resetPassword: async (data: PasswordResetConfirmRequest): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/auth/password-reset/confirm"
            : `${env.NEXT_PUBLIC_API_URL}/auth/password-reset/confirm`;

        try {
            if (useProxyApi) {
                const response = await fetch(url, {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({
                        token: data.token,
                        new_password: data.newPassword,
                        confirm_password: data.confirmPassword,
                    }),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Password reset confirm error: ${response.status}`, errorText);
                    throw new Error(`Password reset confirm failed: ${response.status}`);
                }
            } else {
                await api.post(url, {
                    token: data.token,
                    new_password: data.newPassword,
                    confirm_password: data.confirmPassword,
                });
            }
        } catch (error) {
            console.error("Password reset confirm error:", error);
            throw error;
        }
    },

    // Protected endpoints (require authentication)
    getAccount: async (): Promise<Account> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/auth/account"
            : `${env.NEXT_PUBLIC_API_URL}/auth/account`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Get account error: ${response.status}`, errorText);
                    throw new Error(`Get account failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendAccount>;
                return mapAccountResponse(responseData.data);
            } else {
                const response = await api.get<ApiResponse<BackendAccount>>(url);
                return mapAccountResponse(response.data.data);
            }
        } catch (error) {
            console.error("Get account error:", error);
            throw error;
        }
    },

    changePassword: async (data: ChangePasswordRequest): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/auth/password"
            : `${env.NEXT_PUBLIC_API_URL}/auth/password`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify({
                        current_password: data.currentPassword,
                        new_password: data.newPassword,
                        confirm_password: data.confirmPassword,
                    }),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Change password error: ${response.status}`, errorText);
                    throw new Error(`Change password failed: ${response.status}`);
                }
            } else {
                await api.post(url, {
                    current_password: data.currentPassword,
                    new_password: data.newPassword,
                    confirm_password: data.confirmPassword,
                });
            }
        } catch (error) {
            console.error("Change password error:", error);
            throw error;
        }
    },

    // Admin endpoints - Role management
    createRole: async (data: CreateRoleRequest): Promise<Role> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/auth/roles"
            : `${env.NEXT_PUBLIC_API_URL}/auth/roles`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(data),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Create role error: ${response.status}`, errorText);
                    throw new Error(`Create role failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendRole>;
                return mapRoleResponse(responseData.data);
            } else {
                const response = await api.post<ApiResponse<BackendRole>>(url, data);
                return mapRoleResponse(response.data.data);
            }
        } catch (error) {
            console.error("Create role error:", error);
            throw error;
        }
    },

    getRoles: async (filters?: { name?: string }): Promise<Role[]> => {
        const params = new URLSearchParams();
        if (filters?.name) params.append("name", filters.name);

        const useProxyApi = typeof window !== "undefined";
        let url = useProxyApi
            ? "/api/auth/roles"
            : `${env.NEXT_PUBLIC_API_URL}/auth/roles`;

        const queryString = params.toString();
        if (queryString) {
            url += `?${queryString}`;
        }

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Get roles error: ${response.status}`, errorText);
                    throw new Error(`Get roles failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendRole[]>;
                return responseData.data.map(mapRoleResponse);
            } else {
                const response = await api.get<ApiResponse<BackendRole[]>>(url, { params });
                return response.data.data.map(mapRoleResponse);
            }
        } catch (error) {
            console.error("Get roles error:", error);
            throw error;
        }
    },

    getRole: async (id: string): Promise<Role> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/roles/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/auth/roles/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Get role error: ${response.status}`, errorText);
                    throw new Error(`Get role failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendRole>;
                return mapRoleResponse(responseData.data);
            } else {
                const response = await api.get<ApiResponse<BackendRole>>(url);
                return mapRoleResponse(response.data.data);
            }
        } catch (error) {
            console.error("Get role error:", error);
            throw error;
        }
    },

    updateRole: async (id: string, data: UpdateRoleRequest): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/roles/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/auth/roles/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "PUT",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(data),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Update role error: ${response.status}`, errorText);
                    throw new Error(`Update role failed: ${response.status}`);
                }
            } else {
                await api.put(url, data);
            }
        } catch (error) {
            console.error("Update role error:", error);
            throw error;
        }
    },

    deleteRole: async (id: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/roles/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/auth/roles/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "DELETE",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Delete role error: ${response.status}`, errorText);
                    throw new Error(`Delete role failed: ${response.status}`);
                }
            } else {
                await api.delete(url);
            }
        } catch (error) {
            console.error("Delete role error:", error);
            throw error;
        }
    },

    getRolePermissions: async (roleId: string): Promise<Permission[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/roles/${roleId}/permissions`
            : `${env.NEXT_PUBLIC_API_URL}/auth/roles/${roleId}/permissions`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Get role permissions error: ${response.status}`, errorText);
                    throw new Error(`Get role permissions failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendPermission[]>;
                return responseData.data.map(mapPermissionResponse);
            } else {
                const response = await api.get<ApiResponse<BackendPermission[]>>(url);
                return response.data.data.map(mapPermissionResponse);
            }
        } catch (error) {
            console.error("Get role permissions error:", error);
            throw error;
        }
    },

    assignPermissionToRole: async (roleId: string, permissionId: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/roles/${roleId}/permissions/${permissionId}`
            : `${env.NEXT_PUBLIC_API_URL}/auth/roles/${roleId}/permissions/${permissionId}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Assign permission to role error: ${response.status}`, errorText);
                    throw new Error(`Assign permission to role failed: ${response.status}`);
                }
            } else {
                await api.post(url);
            }
        } catch (error) {
            console.error("Assign permission to role error:", error);
            throw error;
        }
    },

    removePermissionFromRole: async (roleId: string, permissionId: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/roles/${roleId}/permissions/${permissionId}`
            : `${env.NEXT_PUBLIC_API_URL}/auth/roles/${roleId}/permissions/${permissionId}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "DELETE",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Remove permission from role error: ${response.status}`, errorText);
                    throw new Error(`Remove permission from role failed: ${response.status}`);
                }
            } else {
                await api.delete(url);
            }
        } catch (error) {
            console.error("Remove permission from role error:", error);
            throw error;
        }
    },

    // Admin endpoints - Permission management
    createPermission: async (data: CreatePermissionRequest): Promise<Permission> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/auth/permissions"
            : `${env.NEXT_PUBLIC_API_URL}/auth/permissions`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(data),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Create permission error: ${response.status}`, errorText);
                    throw new Error(`Create permission failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendPermission>;
                return mapPermissionResponse(responseData.data);
            } else {
                const response = await api.post<ApiResponse<BackendPermission>>(url, data);
                return mapPermissionResponse(response.data.data);
            }
        } catch (error) {
            console.error("Create permission error:", error);
            throw error;
        }
    },

    getPermissions: async (filters?: { resource?: string; action?: string }): Promise<Permission[]> => {
        const params = new URLSearchParams();
        if (filters?.resource) params.append("resource", filters.resource);
        if (filters?.action) params.append("action", filters.action);

        const useProxyApi = typeof window !== "undefined";
        let url = useProxyApi
            ? "/api/auth/permissions"
            : `${env.NEXT_PUBLIC_API_URL}/auth/permissions`;

        const queryString = params.toString();
        if (queryString) {
            url += `?${queryString}`;
        }

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Get permissions error: ${response.status}`, errorText);
                    throw new Error(`Get permissions failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendPermission[]>;
                return responseData.data.map(mapPermissionResponse);
            } else {
                const response = await api.get<ApiResponse<BackendPermission[]>>(url, { params });
                return response.data.data.map(mapPermissionResponse);
            }
        } catch (error) {
            console.error("Get permissions error:", error);
            throw error;
        }
    },

    getPermission: async (id: string): Promise<Permission> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/permissions/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/auth/permissions/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Get permission error: ${response.status}`, errorText);
                    throw new Error(`Get permission failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendPermission>;
                return mapPermissionResponse(responseData.data);
            } else {
                const response = await api.get<ApiResponse<BackendPermission>>(url);
                return mapPermissionResponse(response.data.data);
            }
        } catch (error) {
            console.error("Get permission error:", error);
            throw error;
        }
    },

    updatePermission: async (id: string, data: UpdatePermissionRequest): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/permissions/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/auth/permissions/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "PUT",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(data),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Update permission error: ${response.status}`, errorText);
                    throw new Error(`Update permission failed: ${response.status}`);
                }
            } else {
                await api.put(url, data);
            }
        } catch (error) {
            console.error("Update permission error:", error);
            throw error;
        }
    },

    deletePermission: async (id: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/permissions/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/auth/permissions/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "DELETE",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Delete permission error: ${response.status}`, errorText);
                    throw new Error(`Delete permission failed: ${response.status}`);
                }
            } else {
                await api.delete(url);
            }
        } catch (error) {
            console.error("Delete permission error:", error);
            throw error;
        }
    },

    // Admin endpoints - Account management
    getAccounts: async (filters?: { email?: string; active?: boolean }): Promise<Account[]> => {
        const params = new URLSearchParams();
        if (filters?.email) params.append("email", filters.email);
        if (filters?.active !== undefined) params.append("active", filters.active.toString());

        const useProxyApi = typeof window !== "undefined";
        let url = useProxyApi
            ? "/api/auth/accounts"
            : `${env.NEXT_PUBLIC_API_URL}/auth/accounts`;

        const queryString = params.toString();
        if (queryString) {
            url += `?${queryString}`;
        }

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Get accounts error: ${response.status}`, errorText);
                    throw new Error(`Get accounts failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendAccount[]>;
                return responseData.data.map(mapAccountResponse);
            } else {
                const response = await api.get<ApiResponse<BackendAccount[]>>(url, { params });
                return response.data.data.map(mapAccountResponse);
            }
        } catch (error) {
            console.error("Get accounts error:", error);
            throw error;
        }
    },

    getAccountById: async (id: string): Promise<Account> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/accounts/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/auth/accounts/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Get account error: ${response.status}`, errorText);
                    throw new Error(`Get account failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendAccount>;
                return mapAccountResponse(responseData.data);
            } else {
                const response = await api.get<ApiResponse<BackendAccount>>(url);
                return mapAccountResponse(response.data.data);
            }
        } catch (error) {
            console.error("Get account error:", error);
            throw error;
        }
    },

    updateAccount: async (id: string, data: UpdateAccountRequest): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/accounts/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/auth/accounts/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "PUT",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(data),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Update account error: ${response.status}`, errorText);
                    throw new Error(`Update account failed: ${response.status}`);
                }
            } else {
                await api.put(url, data);
            }
        } catch (error) {
            console.error("Update account error:", error);
            throw error;
        }
    },

    activateAccount: async (id: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/accounts/${id}/activate`
            : `${env.NEXT_PUBLIC_API_URL}/auth/accounts/${id}/activate`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "PUT",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Activate account error: ${response.status}`, errorText);
                    throw new Error(`Activate account failed: ${response.status}`);
                }
            } else {
                await api.put(url);
            }
        } catch (error) {
            console.error("Activate account error:", error);
            throw error;
        }
    },

    deactivateAccount: async (id: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/accounts/${id}/deactivate`
            : `${env.NEXT_PUBLIC_API_URL}/auth/accounts/${id}/deactivate`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "PUT",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Deactivate account error: ${response.status}`, errorText);
                    throw new Error(`Deactivate account failed: ${response.status}`);
                }
            } else {
                await api.put(url);
            }
        } catch (error) {
            console.error("Deactivate account error:", error);
            throw error;
        }
    },

    getAccountsByRole: async (roleName: string): Promise<Account[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/accounts/by-role/${roleName}`
            : `${env.NEXT_PUBLIC_API_URL}/auth/accounts/by-role/${roleName}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Get accounts by role error: ${response.status}`, errorText);
                    throw new Error(`Get accounts by role failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendAccount[]>;
                return responseData.data.map(mapAccountResponse);
            } else {
                const response = await api.get<ApiResponse<BackendAccount[]>>(url);
                return response.data.data.map(mapAccountResponse);
            }
        } catch (error) {
            console.error("Get accounts by role error:", error);
            throw error;
        }
    },

    assignRoleToAccount: async (accountId: string, roleId: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/accounts/${accountId}/roles/${roleId}`
            : `${env.NEXT_PUBLIC_API_URL}/auth/accounts/${accountId}/roles/${roleId}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Assign role to account error: ${response.status}`, errorText);
                    throw new Error(`Assign role to account failed: ${response.status}`);
                }
            } else {
                await api.post(url);
            }
        } catch (error) {
            console.error("Assign role to account error:", error);
            throw error;
        }
    },

    removeRoleFromAccount: async (accountId: string, roleId: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/accounts/${accountId}/roles/${roleId}`
            : `${env.NEXT_PUBLIC_API_URL}/auth/accounts/${accountId}/roles/${roleId}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "DELETE",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Remove role from account error: ${response.status}`, errorText);
                    throw new Error(`Remove role from account failed: ${response.status}`);
                }
            } else {
                await api.delete(url);
            }
        } catch (error) {
            console.error("Remove role from account error:", error);
            throw error;
        }
    },

    getAccountRoles: async (accountId: string): Promise<Role[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/accounts/${accountId}/roles`
            : `${env.NEXT_PUBLIC_API_URL}/auth/accounts/${accountId}/roles`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Get account roles error: ${response.status}`, errorText);
                    throw new Error(`Get account roles failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendRole[]>;
                return responseData.data.map(mapRoleResponse);
            } else {
                const response = await api.get<ApiResponse<BackendRole[]>>(url);
                return response.data.data.map(mapRoleResponse);
            }
        } catch (error) {
            console.error("Get account roles error:", error);
            throw error;
        }
    },

    getAccountPermissions: async (accountId: string): Promise<Permission[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/accounts/${accountId}/permissions`
            : `${env.NEXT_PUBLIC_API_URL}/auth/accounts/${accountId}/permissions`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Get account permissions error: ${response.status}`, errorText);
                    throw new Error(`Get account permissions failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendPermission[]>;
                return responseData.data.map(mapPermissionResponse);
            } else {
                const response = await api.get<ApiResponse<BackendPermission[]>>(url);
                return response.data.data.map(mapPermissionResponse);
            }
        } catch (error) {
            console.error("Get account permissions error:", error);
            throw error;
        }
    },

    grantPermissionToAccount: async (accountId: string, permissionId: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/accounts/${accountId}/permissions/${permissionId}/grant`
            : `${env.NEXT_PUBLIC_API_URL}/auth/accounts/${accountId}/permissions/${permissionId}/grant`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Grant permission to account error: ${response.status}`, errorText);
                    throw new Error(`Grant permission to account failed: ${response.status}`);
                }
            } else {
                await api.post(url);
            }
        } catch (error) {
            console.error("Grant permission to account error:", error);
            throw error;
        }
    },

    denyPermissionToAccount: async (accountId: string, permissionId: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/accounts/${accountId}/permissions/${permissionId}/deny`
            : `${env.NEXT_PUBLIC_API_URL}/auth/accounts/${accountId}/permissions/${permissionId}/deny`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Deny permission to account error: ${response.status}`, errorText);
                    throw new Error(`Deny permission to account failed: ${response.status}`);
                }
            } else {
                await api.post(url);
            }
        } catch (error) {
            console.error("Deny permission to account error:", error);
            throw error;
        }
    },

    removePermissionFromAccount: async (accountId: string, permissionId: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/accounts/${accountId}/permissions/${permissionId}`
            : `${env.NEXT_PUBLIC_API_URL}/auth/accounts/${accountId}/permissions/${permissionId}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "DELETE",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Remove permission from account error: ${response.status}`, errorText);
                    throw new Error(`Remove permission from account failed: ${response.status}`);
                }
            } else {
                await api.delete(url);
            }
        } catch (error) {
            console.error("Remove permission from account error:", error);
            throw error;
        }
    },

    // Admin endpoints - Token management
    getActiveTokens: async (accountId: string): Promise<Token[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/accounts/${accountId}/tokens`
            : `${env.NEXT_PUBLIC_API_URL}/auth/accounts/${accountId}/tokens`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Get active tokens error: ${response.status}`, errorText);
                    throw new Error(`Get active tokens failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendToken[]>;
                return responseData.data.map(mapTokenResponse);
            } else {
                const response = await api.get<ApiResponse<BackendToken[]>>(url);
                return response.data.data.map(mapTokenResponse);
            }
        } catch (error) {
            console.error("Get active tokens error:", error);
            throw error;
        }
    },

    revokeAllTokens: async (accountId: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/accounts/${accountId}/tokens`
            : `${env.NEXT_PUBLIC_API_URL}/auth/accounts/${accountId}/tokens`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "DELETE",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Revoke all tokens error: ${response.status}`, errorText);
                    throw new Error(`Revoke all tokens failed: ${response.status}`);
                }
            } else {
                await api.delete(url);
            }
        } catch (error) {
            console.error("Revoke all tokens error:", error);
            throw error;
        }
    },

    cleanupExpiredTokens: async (): Promise<number> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/auth/tokens/expired"
            : `${env.NEXT_PUBLIC_API_URL}/auth/tokens/expired`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "DELETE",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Cleanup expired tokens error: ${response.status}`, errorText);
                    throw new Error(`Cleanup expired tokens failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<TokenCleanupResponse>;
                return responseData.data.cleaned_tokens;
            } else {
                const response = await api.delete<ApiResponse<TokenCleanupResponse>>(url);
                return response.data.data.cleaned_tokens;
            }
        } catch (error) {
            console.error("Cleanup expired tokens error:", error);
            throw error;
        }
    },

    // Admin endpoints - Parent account management
    createParentAccount: async (data: CreateParentAccountRequest): Promise<ParentAccount> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/auth/parent-accounts"
            : `${env.NEXT_PUBLIC_API_URL}/auth/parent-accounts`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify({
                        email: data.email,
                        username: data.username,
                        password: data.password,
                        confirm_password: data.confirmPassword,
                    }),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Create parent account error: ${response.status}`, errorText);
                    throw new Error(`Create parent account failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendParentAccount>;
                return mapParentAccountResponse(responseData.data);
            } else {
                const response = await api.post<ApiResponse<BackendParentAccount>>(url, {
                    email: data.email,
                    username: data.username,
                    password: data.password,
                    confirm_password: data.confirmPassword,
                });
                return mapParentAccountResponse(response.data.data);
            }
        } catch (error) {
            console.error("Create parent account error:", error);
            throw error;
        }
    },

    getParentAccounts: async (filters?: { email?: string; active?: boolean }): Promise<ParentAccount[]> => {
        const params = new URLSearchParams();
        if (filters?.email) params.append("email", filters.email);
        if (filters?.active !== undefined) params.append("active", filters.active.toString());

        const useProxyApi = typeof window !== "undefined";
        let url = useProxyApi
            ? "/api/auth/parent-accounts"
            : `${env.NEXT_PUBLIC_API_URL}/auth/parent-accounts`;

        const queryString = params.toString();
        if (queryString) {
            url += `?${queryString}`;
        }

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Get parent accounts error: ${response.status}`, errorText);
                    throw new Error(`Get parent accounts failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendParentAccount[]>;
                return responseData.data.map(mapParentAccountResponse);
            } else {
                const response = await api.get<ApiResponse<BackendParentAccount[]>>(url, { params });
                return response.data.data.map(mapParentAccountResponse);
            }
        } catch (error) {
            console.error("Get parent accounts error:", error);
            throw error;
        }
    },

    getParentAccount: async (id: string): Promise<ParentAccount> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/parent-accounts/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/auth/parent-accounts/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Get parent account error: ${response.status}`, errorText);
                    throw new Error(`Get parent account failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendParentAccount>;
                return mapParentAccountResponse(responseData.data);
            } else {
                const response = await api.get<ApiResponse<BackendParentAccount>>(url);
                return mapParentAccountResponse(response.data.data);
            }
        } catch (error) {
            console.error("Get parent account error:", error);
            throw error;
        }
    },

    updateParentAccount: async (id: string, data: { email: string; username?: string }): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/parent-accounts/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/auth/parent-accounts/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "PUT",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(data),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Update parent account error: ${response.status}`, errorText);
                    throw new Error(`Update parent account failed: ${response.status}`);
                }
            } else {
                await api.put(url, data);
            }
        } catch (error) {
            console.error("Update parent account error:", error);
            throw error;
        }
    },

    activateParentAccount: async (id: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/parent-accounts/${id}/activate`
            : `${env.NEXT_PUBLIC_API_URL}/auth/parent-accounts/${id}/activate`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "PUT",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Activate parent account error: ${response.status}`, errorText);
                    throw new Error(`Activate parent account failed: ${response.status}`);
                }
            } else {
                await api.put(url);
            }
        } catch (error) {
            console.error("Activate parent account error:", error);
            throw error;
        }
    },

    deactivateParentAccount: async (id: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/auth/parent-accounts/${id}/deactivate`
            : `${env.NEXT_PUBLIC_API_URL}/auth/parent-accounts/${id}/deactivate`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "PUT",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Deactivate parent account error: ${response.status}`, errorText);
                    throw new Error(`Deactivate parent account failed: ${response.status}`);
                }
            } else {
                await api.put(url);
            }
        } catch (error) {
            console.error("Deactivate parent account error:", error);
            throw error;
        }
    },
};