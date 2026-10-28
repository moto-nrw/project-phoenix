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

const { auth: rawAuth, handlers, signIn } = NextAuth(tenantAuthConfig);
const { auth, uncachedAuth, withAuthResponse } = createResponseAwareAuth(
  rawAuth,
  "tenant",
);

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
