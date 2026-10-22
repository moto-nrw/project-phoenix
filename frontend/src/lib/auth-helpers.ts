// lib/auth-helpers.ts
// Type definitions for auth entities

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
  admin: "Administrator",
  user: "Betreuer",
  guardian: "Erziehungsberechtigte",
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

// System role display names and descriptions (German localization)
// Backend uses English names for authorization logic, frontend displays German
const SYSTEM_ROLE_TRANSLATIONS: Record<
  string,
  { name: string; description: string }
> = {
  admin: {
    name: "Administrator",
    description: "Systemadministrator mit vollem Zugriff",
  },
  user: {
    name: "Betreuer",
    description: "Pädagogische Fachkraft mit Betreuungsrechten",
  },
  guest: {
    name: "Gast",
    description: "Eingeschränkter Zugriff für nicht authentifizierte Benutzer",
  },
  guardian: {
    name: "Erziehungsberechtigter",
    description: "Eingeschränkter Zugriff auf Daten der eigenen Kinder",
  },
  lehrkraft: {
    name: "Lehrkraft",
    description: "Lesezugriff auf die Tagesansicht der zugewiesenen Klassen",
  },
};

/**
 * Get the German display name for a system role.
 * Falls back to the original name if no translation exists.
 */
export function getRoleDisplayName(roleName: string): string {
  const key = roleName.trim().toLowerCase();
  return (
    SYSTEM_ROLE_TRANSLATIONS[key]?.name ??
    PROVISIONED_SCHOOL_ROLE_LABELS[key] ??
    roleName
  );
}

/**
 * The school role every school gets for external caregivers (#4573). The
 * role store keeps custom names lowercase, so it arrives as „extern“.
 */
export const EXTERNAL_CAREGIVER_ROLE_NAME = "extern";

// School roles moto itself provisions in every school. Their stored name is
// lowercase; the display keeps the spelling the help and the forms use.
const PROVISIONED_SCHOOL_ROLE_LABELS: Record<string, string> = {
  [EXTERNAL_CAREGIVER_ROLE_NAME]: "Extern",
};

/**
 * Get the German description for a system role.
 * Falls back to the original description if no translation exists.
 */
export function getRoleDisplayDescription(
  roleName: string,
  originalDescription: string,
): string {
  return (
    SYSTEM_ROLE_TRANSLATIONS[roleName.toLowerCase()]?.description ??
    originalDescription
  );
}

// Labels of school roles that are not system roles. The demo school seeds
// both (backend/seed/api/demo_roles.go); role names are stored lowercase, so
// "ogs-leitung" would otherwise read as „Ogs-leitung".
const SCHOOL_ROLE_LABELS: Record<string, string> = {
  "ogs-leitung": "OGS-Leitung",
  betreuungskraft: "Betreuungskraft",
  [EXTERNAL_CAREGIVER_ROLE_NAME]: "Extern",
};

// "teacher" is the retired predecessor of "user". It can no longer be
// assigned, but existing sessions still carry it and must not be presented as
// a school-specific role.
const SYSTEM_ROLE_NAMES = new Set([
  ...Object.keys(SYSTEM_ROLE_TRANSLATIONS),
  "teacher",
]);

/**
 * The role label next to the name in the staff portal's header (#3891). An
 * administrator stays „Admin"; otherwise the school's own role names the
 * person better than the system role behind it, so the OGS lead of the demo
 * reads „OGS-Leitung", not „Betreuer".
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
    isSystem: hasRoleOrigins
      ? roleIsSystem[index]!
      : SYSTEM_ROLE_NAMES.has(name.toLowerCase()),
  }));
  if (roles.some((role) => role.isSystem && role.name === "admin")) {
    return "Admin";
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
  return systemRole === "teacher"
    ? "Betreuer"
    : systemRole
      ? getRoleDisplayName(systemRole)
      : "Betreuer";
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
