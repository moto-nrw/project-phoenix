// lib/auth-helpers.ts
// Type definitions for auth entities

export interface BackendAccount {
    ID: number;
    Email: string;
    Username?: string;
    Name?: string;
    Active: boolean;
    Roles?: BackendRole[];
    Permissions?: BackendPermission[];
    CreatedAt: string;
    UpdatedAt: string;
}

export interface BackendRole {
    ID: number;
    Name: string;
    Description: string;
    CreatedAt: string;
    UpdatedAt: string;
    Permissions?: BackendPermission[];
}

export interface BackendPermission {
    id: number;
    name: string;
    description: string;
    resource: string;
    action: string;
    created_at: string;
    updated_at: string;
}

export interface BackendToken {
    ID: number;
    Token: string;
    Expiry: string;
    Mobile: boolean;
    Identifier?: string;
    CreatedAt: string;
}

export interface BackendParentAccount {
    ID: number;
    Email: string;
    Username?: string;
    Active: boolean;
    CreatedAt: string;
    UpdatedAt: string;
}

// Frontend types
export interface Account {
    id: string;
    email: string;
    username?: string;
    name?: string;
    active: boolean;
    roles?: Role[];
    permissions?: Permission[];
    createdAt: string;
    updatedAt: string;
}

export interface Role {
    id: string;
    name: string;
    description: string;
    createdAt: string;
    updatedAt: string;
    permissions?: Permission[];
}

export interface Permission {
    id: string;
    name: string;
    description: string;
    resource: string;
    action: string;
    createdAt: string;
    updatedAt: string;
}

export interface Token {
    id: string;
    token: string;
    expiry: string;
    mobile: boolean;
    identifier?: string;
    createdAt: string;
}

export interface ParentAccount {
    id: string;
    email: string;
    username?: string;
    active: boolean;
    createdAt: string;
    updatedAt: string;
}

// Transformation functions
export function mapAccountResponse(backendAccount: BackendAccount): Account {
    return {
        id: String(backendAccount.ID),
        email: backendAccount.Email,
        username: backendAccount.Username,
        name: backendAccount.Name,
        active: backendAccount.Active,
        roles: backendAccount.Roles?.map(mapRoleResponse),
        permissions: backendAccount.Permissions?.map(mapPermissionResponse),
        createdAt: backendAccount.CreatedAt,
        updatedAt: backendAccount.UpdatedAt,
    };
}

// Flexible role interface for handling mixed case API responses
interface FlexibleRoleData {
    ID?: number;
    id?: number;
    Name?: string;
    name?: string;
    Description?: string;
    description?: string;
    CreatedAt?: string;
    created_at?: string;
    UpdatedAt?: string;
    updated_at?: string;
    Permissions?: BackendPermission[];
    permissions?: BackendPermission[];
}

export function mapRoleResponse(backendRole: BackendRole | FlexibleRoleData): Role {
    // Handle both uppercase (BackendRole) and lowercase (from API) field names
    const roleData = backendRole as BackendRole & FlexibleRoleData;
    return {
        id: String(roleData.ID ?? roleData.id ?? 0),
        name: roleData.Name ?? roleData.name ?? '',
        description: roleData.Description ?? roleData.description ?? '',
        createdAt: roleData.CreatedAt ?? roleData.created_at ?? '',
        updatedAt: roleData.UpdatedAt ?? roleData.updated_at ?? '',
        permissions: (roleData.Permissions ?? roleData.permissions)?.map(mapPermissionResponse) ?? undefined,
    };
}

export function mapPermissionResponse(backendPermission: BackendPermission): Permission {
    return {
        id: String(backendPermission.id),
        name: backendPermission.name,
        description: backendPermission.description,
        resource: backendPermission.resource,
        action: backendPermission.action,
        createdAt: backendPermission.created_at,
        updatedAt: backendPermission.updated_at,
    };
}

export function mapTokenResponse(backendToken: BackendToken): Token {
    return {
        id: String(backendToken.ID),
        token: backendToken.Token,
        expiry: backendToken.Expiry,
        mobile: backendToken.Mobile,
        identifier: backendToken.Identifier,
        createdAt: backendToken.CreatedAt,
    };
}

export function mapParentAccountResponse(backendParentAccount: BackendParentAccount): ParentAccount {
    return {
        id: String(backendParentAccount.ID),
        email: backendParentAccount.Email,
        username: backendParentAccount.Username,
        active: backendParentAccount.Active,
        createdAt: backendParentAccount.CreatedAt,
        updatedAt: backendParentAccount.UpdatedAt,
    };
}

// Request/Response types
export interface LoginRequest {
    email: string;
    password: string;
}

export interface RegisterRequest {
    email: string;
    username: string;
    name: string;
    password: string;
    confirmPassword: string;
}

export interface TokenResponse {
    access_token: string;
    refresh_token: string;
}

export interface ChangePasswordRequest {
    currentPassword: string;
    newPassword: string;
    confirmPassword: string;
}

export interface PasswordResetRequest {
    email: string;
}

export interface PasswordResetConfirmRequest {
    token: string;
    newPassword: string;
    confirmPassword: string;
}

export interface CreateRoleRequest {
    name: string;
    description: string;
}

export interface UpdateRoleRequest {
    name: string;
    description: string;
}

export interface CreatePermissionRequest {
    name: string;
    description: string;
    resource: string;
    action: string;
}

export interface UpdatePermissionRequest {
    name: string;
    description: string;
    resource: string;
    action: string;
}

export interface UpdateAccountRequest {
    email: string;
    username?: string;
}

export interface CreateParentAccountRequest {
    email: string;
    username: string;
    password: string;
    confirmPassword: string;
}