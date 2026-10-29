/**
 * Tenant auth instance (default).
 *
 * All existing imports from "~/server/auth" continue to work unchanged.
 * This is the tenant-scoped NextAuth instance with cookies shared across
 * tenant subdomains for tenant-to-tenant switching.
 */

import "server-only";
import NextAuth from "next-auth";

import { tenantAuthConfig } from "./tenant-config";
import { createResponseAwareAuth } from "./route-handler";
import { sessionTenantMismatchResponse } from "./tenant-host-check";

const {
  auth: rawAuth,
  handlers,
  signIn,
  unstable_update,
} = NextAuth(tenantAuthConfig);
const { auth, uncachedAuth, withAuthResponse, mutateSession } =
  createResponseAwareAuth(rawAuth, "tenant");

/**
 * Every tenant BFF route. A session of another school than the request's
 * subdomain is refused before the handler runs (#5130).
 */
const withTenantAuth: typeof withAuthResponse = (handler) =>
  withAuthResponse(async (request, context) => {
    const mismatch = await sessionTenantMismatchResponse(request, request.auth);
    return mismatch ?? handler(request, context);
  });

/** @public uncachedAuth — false positive in knip 6.x, used by route-wrapper.ts */
export { auth, uncachedAuth, handlers, signIn, withTenantAuth };

/** Tokens never cross the BFF response boundary. Auth.js still validates them. */
export async function setTenantSession(
  accessToken: string,
  refreshToken: string,
) {
  await mutateSession(
    () =>
      signIn("credentials", {
        redirect: false,
        internalRefresh: "true",
        token: accessToken,
        refreshToken,
      }),
    (result) =>
      typeof result === "string" &&
      !new URL(result, "http://localhost").searchParams.has("error"),
  );
}

export async function setStaffPreviewSession(preview: {
  accessToken: string;
  expiresIn: number;
  targetAccountId: string;
  targetName: string;
}) {
  await mutateSession(
    () =>
      unstable_update({ previewStart: preview } as Parameters<
        typeof unstable_update
      >[0]),
    (session) =>
      session?.user?.isPreview === true &&
      session.user.previewTargetAccountId === preview.targetAccountId,
  );
}
