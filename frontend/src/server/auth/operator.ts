/**
 * Operator auth instance.
 *
 * Separate NextAuth instance with host-only cookies (no domain),
 * so the operator session is invisible to tenant subdomains.
 *
 * Import from "~/server/auth/operator" in operator-specific code.
 */

import "server-only";
import NextAuth from "next-auth";

import { operatorAuthConfig } from "./operator-config";
import { createSessionWriter } from "./session-writer";
import { createResponseAwareAuth } from "./route-handler";

const {
  auth: rawOperatorAuth,
  handlers: operatorHandlers,
  signIn,
} = NextAuth(operatorAuthConfig);
const {
  auth: operatorAuth,
  uncachedAuth: uncachedOperatorAuth,
  withAuthResponse: withOperatorAuth,
  withSessionResponse: withOperatorSessionResponse,
  mutateSession,
} = createResponseAwareAuth(rawOperatorAuth, "operator");

export {
  operatorAuth,
  uncachedOperatorAuth,
  operatorHandlers,
  withOperatorAuth,
};

export const setOperatorSession = createSessionWriter(
  mutateSession,
  signIn,
  "operator-credentials",
);
export { withOperatorSessionResponse };
