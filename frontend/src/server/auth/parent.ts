/**
 * Parent auth instance.
 *
 * Separate NextAuth instance with host-only cookies (no domain), so the
 * parent session is invisible to both tenant and operator subdomains.
 *
 * Import from "~/server/auth/parent" in parent-app code.
 */

import "server-only";
import NextAuth from "next-auth";

import { parentAuthConfig } from "./parent-config";
import { createSessionWriter } from "./session-writer";
import { createResponseAwareAuth } from "./route-handler";

const {
  auth: rawParentAuth,
  handlers: parentHandlers,
  signIn,
} = NextAuth(parentAuthConfig);
const {
  auth: parentAuth,
  uncachedAuth: uncachedParentAuth,
  withAuthResponse: withParentAuth,
  withSessionResponse: withParentSessionResponse,
  mutateSession,
} = createResponseAwareAuth(rawParentAuth, "parent");

export { parentAuth, uncachedParentAuth, parentHandlers, withParentAuth };

export const setParentSession = createSessionWriter(
  mutateSession,
  signIn,
  "parent-credentials",
);
export { withParentSessionResponse };
