import type { Session } from "next-auth";
import { getToken } from "next-auth/jwt";
import { headers } from "next/headers";
import { env } from "~/env";
import { ACCESS_TOKEN_REFRESH_BUFFER_MS } from "~/server/auth/shared";
import { tenantAuthConfig } from "~/server/auth/tenant-config";

/**
 * Reads the encrypted Auth.js JWT without running its refresh callback.
 * Server Components cannot persist a rotated session cookie, so calling
 * auth() here could consume a refresh token and strand the browser on the old
 * cookie (#1938). This snapshot hydrates SessionProvider and the shell preload
 * without exposing the refresh token; later provider polls still use the
 * response-aware /api/auth/session route.
 */
export async function readTenantSessionSnapshot(): Promise<Session | null> {
  const cookieName = tenantAuthConfig.cookies?.sessionToken?.name;
  const token = await getToken({
    req: { headers: new Headers(await headers()) },
    secret: env.NEXTAUTH_SECRET,
    secureCookie: new URL(env.NEXTAUTH_URL).protocol === "https:",
    ...(cookieName ? { cookieName, salt: cookieName } : {}),
  });

  if (
    typeof token?.id !== "string" ||
    typeof token.token !== "string" ||
    typeof token.tenantId !== "number" ||
    typeof token.tokenExpiry !== "number" ||
    token.tokenExpiry <= Date.now() + ACCESS_TOKEN_REFRESH_BUFFER_MS ||
    (typeof token.refreshTokenExpiry === "number" &&
      token.refreshTokenExpiry <= Date.now()) ||
    token.error
  ) {
    return null;
  }

  return {
    expires: new Date((token.exp ?? 0) * 1000).toISOString(),
    user: {
      id: token.id,
      name: token.name,
      email: token.email,
      image: token.picture,
      token: token.token,
      roles: Array.isArray(token.roles) ? token.roles : undefined,
      permissions: Array.isArray(token.permissions)
        ? token.permissions
        : undefined,
      firstName:
        typeof token.firstName === "string" ? token.firstName : undefined,
      isAdmin: token.isAdmin === true,
      tenantId: token.tenantId,
      orgId: typeof token.orgId === "number" ? token.orgId : undefined,
      scope: typeof token.scope === "string" ? token.scope : undefined,
      isPreview: typeof token.previewTargetAccountId === "string",
      previewTargetName:
        typeof token.previewTargetName === "string"
          ? token.previewTargetName
          : undefined,
      previewTargetAccountId:
        typeof token.previewTargetAccountId === "string"
          ? token.previewTargetAccountId
          : undefined,
    },
  };
}
