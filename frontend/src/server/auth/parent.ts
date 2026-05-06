/**
 * Parent auth instance.
 *
 * Separate NextAuth instance with host-only cookies (no domain), so the
 * parent session is invisible to both tenant and operator subdomains.
 *
 * Import from "~/server/auth/parent" in parent-app code.
 */

import NextAuth from "next-auth";
import { cache } from "react";

import { parentAuthConfig } from "./parent-config";

const {
  auth: uncachedParentAuth,
  handlers: parentHandlers,
  signIn: parentSignIn,
} = NextAuth(parentAuthConfig);

const parentAuth = cache(uncachedParentAuth);

export { parentAuth, uncachedParentAuth, parentHandlers, parentSignIn };
