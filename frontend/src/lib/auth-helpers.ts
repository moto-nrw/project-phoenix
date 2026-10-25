// lib/auth-helpers.ts
// Type definitions for auth entities

import {
  SYSTEM_ROLE_LABELS,
  isSystemRoleName,
  systemRoleDescription,
  systemRoleLabel,
} from "~/lib/role-labels";

export interface BackendRole {
  ID: string | number;
  Name: string;
  Description: string;
  IsSystem: boolean;
  BaseRole?: string;
  CreatedAt: string;
  UpdatedAt: string;
  Permissions?: BackendPermission[];
}

export interface BackendPermission {
  id: string | number;
  name: string;
  description: string;
  resource: string;
  action: string;
  created_at: string;
  updated_at: string;
}

// Frontend types
/** German display labels for system base roles used in announcement targeting. */
export const BASE_ROLE_LABELS: Record<string, string> = {
  admin: SYSTEM_ROLE_LABELS.admin.name,
  user: SYSTEM_ROLE_LABELS.user.name,
  guardian: SYSTEM_ROLE_LABELS.guardian.name,
};

/** Returns the German display label for a base role, or a fallback for unmapped/missing values. */
export function getBaseRoleLabel(baseRole?: string): string {
  if (!baseRole) return "Keine Zuordnung";
  return BASE_ROLE_LABELS[baseRole] ?? baseRole;
}

export interface Role {
  id: string;
  name: string;
  description: string;
  isSystem: boolean;
  baseRole?: string;
  /** Made from the template „Wie Lehrkraft“: works in moto schule (#4574). */
  schoolPortal?: boolean;
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

// Transformation functions
// Flexible role interface for handling mixed case API responses
interface FlexibleRoleData {
  ID?: string | number;
  id?: string | number;
  Name?: string;
  name?: string;
  Description?: string;
  description?: string;
  IsSystem?: boolean;
  is_system?: boolean;
  BaseRole?: string;
  base_role?: string;
  school_portal?: boolean;
  CreatedAt?: string;
  created_at?: string;
  UpdatedAt?: string;
  updated_at?: string;
  Permissions?: BackendPermission[];
  permissions?: BackendPermission[];
}

export function mapRoleResponse(
  backendRole: BackendRole | FlexibleRoleData,
): Role {
  // Handle both uppercase (BackendRole) and lowercase (from API) field names
  const roleData = backendRole as BackendRole & FlexibleRoleData;
  return {
    id: String(roleData.ID ?? roleData.id ?? 0),
    name: roleData.Name ?? roleData.name ?? "",
    description: roleData.Description ?? roleData.description ?? "",
    isSystem: roleData.IsSystem ?? roleData.is_system ?? false,
    baseRole: roleData.BaseRole ?? roleData.base_role,
    schoolPortal: roleData.school_portal ?? false,
    createdAt: roleData.CreatedAt ?? roleData.created_at ?? "",
    updatedAt: roleData.UpdatedAt ?? roleData.updated_at ?? "",
    permissions:
      (roleData.Permissions ?? roleData.permissions)?.map(
        mapPermissionResponse,
      ) ?? undefined,
  };
}

export function mapPermissionResponse(
  backendPermission: BackendPermission,
): Permission {
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

// Labels of school roles that are not system roles. The demo school seeds
// both (backend/internal/devtools/seedapi/demo_roles.go); role names are
// stored lowercase, so "ogs-leitung" would otherwise read as „Ogs-leitung“
// in the header and as "ogs-leitung" in every role list.
const SCHOOL_ROLE_LABELS: Record<string, string> = {
  "ogs-leitung": "OGS-Leitung",
  betreuungskraft: "Betreuungskraft",
};

/**
 * Get the German display name for a system role (`~/lib/role-labels`) or a
 * demo school role. Falls back to the original name if no translation exists.
 */
export function getRoleDisplayName(roleName: string): string {
  return (
    systemRoleLabel(roleName) ??
    SCHOOL_ROLE_LABELS[roleName.toLowerCase()] ??
    roleName
  );
}

/**
 * How an external caregiver is marked in lists, badges and select options:
 * „Extern“. An attribute of the person, not a role (ADR 0091).
 */
export const EXTERNAL_CAREGIVER_LABEL = "Extern";

/**
 * Get the German description for a system role.
 * Falls back to the original description if no translation exists.
 */
export function getRoleDisplayDescription(
  roleName: string,
  originalDescription: string,
): string {
  return systemRoleDescription(roleName) ?? originalDescription;
}

/**
 * The role label next to the name in the staff portal's header (#3891). An
 * administrator reads „Verwaltung"; otherwise the school's own role names the
 * person better than the system role behind it, so the OGS lead of the demo
 * reads „OGS-Leitung", not „Betreuung".
 */
export function getAccountRoleLabel(
  roleNames: readonly string[],
  roleIsSystem?: readonly boolean[],
): string {
  const hasRoleOrigins = roleIsSystem?.length === roleNames.length;
  const roles = roleNames.map((name, index) => ({
    name: name.toLowerCase(),
    // Sessions issued before role origins were added retain the earlier
    // name-based display until their next refresh.
    isSystem: hasRoleOrigins ? roleIsSystem[index]! : isSystemRoleName(name),
  }));
  if (roles.some((role) => role.isSystem && role.name === "admin")) {
    return SYSTEM_ROLE_LABELS.admin.name;
  }
  const schoolRole = roles.find((role) => !role.isSystem)?.name;
  if (schoolRole) {
    return (
      SCHOOL_ROLE_LABELS[schoolRole] ??
      schoolRole.charAt(0).toUpperCase() + schoolRole.slice(1)
    );
  }
  const systemRole = roles.find(
    (role) => role.isSystem && role.name !== "user",
  )?.name;
  return systemRole && systemRole !== "teacher"
    ? getRoleDisplayName(systemRole)
    : SYSTEM_ROLE_LABELS.user.name;
}

export interface RoleOption {
  id: string;
  name: string;
  // Raw backend role name ("lehrkraft", "user", …) — the display `name` is
  // the German label, so flows that must branch on the role (e.g. Lehrkraft
  // gets NO caregiver/teacher profile) compare against this.
  systemName: string;
}

/**
 * System roles that are legacy/relationship-derived and not assignable to
 * staff accounts. "lehrkraft" (#1772) is assignable: an invited Lehrkraft
 * lands in the Klassenansicht of moto schule (#2207), not in this portal.
 * "guest" is assignable as the view-only access (ADR 0091).
 */
const NON_ASSIGNABLE_STAFF_ROLE_NAMES = new Set(["guardian", "teacher"]);

/**
 * Whether a system role may be assigned to a staff account. Shared by every
 * role picker (invitation form, teacher-creation form, role-management modal,
 * operator account creation) so the exclusion list lives in one place.
 */
export function isAssignableStaffRole(
  roleName: string,
  isSystem: boolean,
): boolean {
  return (
    !isSystem || !NON_ASSIGNABLE_STAFF_ROLE_NAMES.has(roleName.toLowerCase())
  );
}

/**
 * Filters a role list down to staff-assignable roles and maps them to
 * display options. Shared by the invitation form, teacher-creation form,
 * and the staff role-management modal to avoid re-deriving this list.
 */
export function toAssignableRoleOptions(roles: Role[]): RoleOption[] {
  return roles
    .filter(
      (role) =>
        isAssignableStaffRole(role.name, role.isSystem) &&
        role.baseRole?.toLowerCase() !== "guardian",
    )
    .map((role) => ({
      id: role.id,
      name: role.name ? getRoleDisplayName(role.name) : `Rolle ${role.id}`,
      systemName: role.name?.toLowerCase() ?? "",
    }));
}

// Request types
export interface PasswordResetConfirmRequest {
  token: string;
  newPassword: string;
  confirmPassword: string;
}
