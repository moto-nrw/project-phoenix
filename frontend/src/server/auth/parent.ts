/**
 * Parent auth instance.
 *
 * Separate NextAuth instance with host-only cookies (no domain), so the
 * parent session is invisible to both tenant and operator subdomains.
 *
 * Import from "~/server/auth/parent" in parent-app code.
 */

import NextAuth from "next-auth";

import { parentAuthConfig } from "./parent-config";
import { createResponseAwareAuth } from "./route-handler";

const { auth: rawParentAuth, handlers: parentHandlers } =
  NextAuth(parentAuthConfig);
const {
  auth: parentAuth,
  uncachedAuth: uncachedParentAuth,
  withAuthResponse: withParentAuth,
} = createResponseAwareAuth(rawParentAuth);

export { parentAuth, uncachedParentAuth, parentHandlers, withParentAuth };
