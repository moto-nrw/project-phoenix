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
import { rejectForeignTenantSession } from "./tenant-host-session";

const { auth: rawAuth, handlers, signIn } = NextAuth(tenantAuthConfig);
// Every tenant BFF route, SSE included, answers a session of another school
// than its host itself (#5130).
const {
  auth,
  uncachedAuth,
  withAuthResponse: withTenantAuth,
} = createResponseAwareAuth(rawAuth, "tenant", {
  rejectSession: rejectForeignTenantSession,
});

/** @public uncachedAuth — false positive in knip 6.x, used by route-wrapper.ts */
export { auth, uncachedAuth, handlers, signIn, withTenantAuth };
