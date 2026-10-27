import type { Session } from "next-auth";
import { getToken } from "next-auth/jwt";
import { headers } from "next/headers";
import { env } from "~/env";
import { ACCESS_TOKEN_REFRESH_BUFFER_MS } from "~/server/auth/shared";
import { tenantAuthConfig } from "~/server/auth/tenant-config";

/**
 * The tenant layout's read of the session cookie. `session` is serialized
 * into the HTML and RSC payload and carries no token (#5005); `accessToken`
 * stays on the server for the shell preload.
 */
export interface TenantSessionSnapshot {
  session: Session;
  accessToken: string;
}

/**
 * Reads the encrypted Auth.js JWT without running its refresh callback.
 * Server Components cannot persist a rotated session cookie, so calling
 * auth() here could consume a refresh token and strand the browser on the old
 * cookie (#1938). The session hydrates SessionProvider without any token;
 * later provider polls still use the response-aware /api/auth/session route.
 */
export async function readTenantSessionSnapshot(): Promise<TenantSessionSnapshot | null> {
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

  const session: Session = {
    expires: new Date((token.exp ?? 0) * 1000).toISOString(),
    user: {
      id: token.id,
      name: token.name,
      email: token.email,
      image: token.picture,
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
  return { session, accessToken: token.token };
}
