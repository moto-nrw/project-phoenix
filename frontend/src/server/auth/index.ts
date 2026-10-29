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
const {
  auth,
  uncachedAuth,
  withAuthResponse,
  mutateSession,
  withSessionResponse: withTenantSessionResponse,
} = createResponseAwareAuth(rawAuth, "tenant");

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
export {
  auth,
  uncachedAuth,
  handlers,
  signIn,
  withTenantAuth,
  withTenantSessionResponse,
};

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
    // Auth.js server signIn throws on refused Credentials. Its return value
    // is the callback URL, which may legitimately contain an `error` query
    // inherited from the referring page; that is not an auth failure.
    (result) => typeof result === "string",
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
