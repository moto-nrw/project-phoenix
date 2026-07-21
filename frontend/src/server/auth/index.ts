/**
 * Tenant auth instance (default).
 *
 * All existing imports from "~/server/auth" continue to work unchanged.
 * This is the tenant-scoped NextAuth instance with cookies shared across
 * tenant subdomains for tenant-to-tenant switching.
 */

import NextAuth from "next-auth";

import { tenantAuthConfig } from "./tenant-config";
import { createResponseAwareAuth } from "./route-handler";

const { auth: rawAuth, handlers, signIn } = NextAuth(tenantAuthConfig);
const {
  auth,
  uncachedAuth,
  withAuthResponse: withTenantAuth,
} = createResponseAwareAuth(rawAuth);

/** @public uncachedAuth — false positive in knip 6.x, used by route-wrapper.ts */
export { auth, uncachedAuth, handlers, signIn, withTenantAuth };
