/**
 * Tenant auth instance (default).
 *
 * All existing imports from "~/server/auth" continue to work unchanged.
 * This is the tenant-scoped NextAuth instance with cookies shared across
 * tenant subdomains for tenant-to-tenant switching.
 */

import NextAuth from "next-auth";
import { cache } from "react";

import { tenantAuthConfig } from "./tenant-config";

const { auth: uncachedAuth, handlers, signIn } = NextAuth(tenantAuthConfig);

const auth = cache(uncachedAuth);

export { auth, uncachedAuth, handlers, signIn };
