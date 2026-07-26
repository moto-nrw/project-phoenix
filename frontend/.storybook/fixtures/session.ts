import type { Session } from "next-auth";

type StorybookSessionUserOverrides = Partial<Session["user"]> & {
  role?: string;
  accessToken?: string;
};

// Mirrors the seeded teacher role (backend migration
// 001007004_add_domain_roles_with_permissions.go). Notably no config:read,
// so config-gated nav items (e.g. Essensplan) stay hidden — overriding roles
// alone is not enough because the default admin:* permission wildcard would
// still grant everything.
const TEACHER_PERMISSIONS = [
  "users:read",
  "users:update",
  "users:list",
  "groups:create",
  "groups:read",
  "groups:update",
  "groups:delete",
  "groups:list",
  "groups:manage",
  "groups:assign",
  "activities:create",
  "activities:read",
  "activities:update",
  "activities:delete",
  "activities:list",
  "activities:manage",
  "activities:enroll",
  "activities:assign",
  "visits:create",
  "visits:read",
  "visits:update",
  "visits:delete",
  "visits:list",
  "rooms:read",
  "rooms:list",
  "substitutions:create",
  "substitutions:read",
  "substitutions:update",
  "substitutions:delete",
  "substitutions:list",
  "substitutions:manage",
  "schedules:read",
  "schedules:list",
  "feedback:read",
  "feedback:list",
];

// A realistic non-admin staff (caregiver) session: role "teacher" takes the
// caregiver nav branch (isCaregiver in ~/lib/auth-utils) and the permission
// set matches what the backend actually grants that role.
export function mockTeacherSessionData(): Session {
  return mockSessionData({
    user: { roles: ["teacher"], permissions: TEACHER_PERMISSIONS },
  });
}

export function mockSessionData(
  overrides?: Partial<{
    user: StorybookSessionUserOverrides;
    expires: string;
  }>,
): Session {
  const {
    accessToken,
    isAdmin,
    permissions,
    refreshToken,
    role,
    roles,
    token,
    ...userOverrides
  } = overrides?.user ?? {};

  const resolvedRoles = roles ?? (role ? [role] : ["admin"]);

  return {
    user: {
      id: "1",
      name: "Test User",
      email: "test@test.com",
      firstName: "Test",
      roles: resolvedRoles,
      permissions: permissions ?? ["admin:*"],
      isAdmin: isAdmin ?? resolvedRoles.includes("admin"),
      token: token ?? accessToken ?? "test-token",
      refreshToken: refreshToken ?? "test-refresh-token",
      tenantId: 1,
      orgId: 1,
      scope: "",
      ...userOverrides,
    },
    expires: overrides?.expires ?? "2099-01-01T00:00:00.000Z",
  };
}
