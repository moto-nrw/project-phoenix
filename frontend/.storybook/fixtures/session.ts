import type { Session } from "next-auth";

type StorybookSessionUserOverrides = Partial<Session["user"]> & {
  role?: string;
  accessToken?: string;
};

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
