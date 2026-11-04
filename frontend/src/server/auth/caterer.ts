/**
 * Caterer auth instance (ADR 0077).
 *
 * Separate NextAuth instance with host-only cookies (no domain), so the
 * caterer session is invisible to every other host.
 *
 * Import from "~/server/auth/caterer" in caterer-app code.
 */

import "server-only";
import NextAuth from "next-auth";

import { catererAuthConfig } from "./caterer-config";
import { createSessionWriter } from "./session-writer";
import { createResponseAwareAuth } from "./route-handler";

const {
  auth: rawCatererAuth,
  handlers: catererHandlers,
  signIn,
} = NextAuth(catererAuthConfig);
const {
  auth: catererAuth,
  uncachedAuth: uncachedCatererAuth,
  withAuthResponse: withCatererAuth,
  withSessionResponse: withCatererSessionResponse,
  mutateSession,
} = createResponseAwareAuth(rawCatererAuth, "caterer");

export { catererAuth, uncachedCatererAuth, catererHandlers, withCatererAuth };

export const setCatererSession = createSessionWriter(
  mutateSession,
  signIn,
  "caterer-credentials",
);
export { withCatererSessionResponse };
