/**
 * Operator auth instance.
 *
 * Separate NextAuth instance with host-only cookies (no domain),
 * so the operator session is invisible to tenant subdomains.
 *
 * Import from "~/server/auth/operator" in operator-specific code.
 */

import NextAuth from "next-auth";

import { operatorAuthConfig } from "./operator-config";
import { createResponseAwareAuth } from "./route-handler";

const { auth: rawOperatorAuth, handlers: operatorHandlers } =
  NextAuth(operatorAuthConfig);
const {
  auth: operatorAuth,
  uncachedAuth: uncachedOperatorAuth,
  withAuthResponse: withOperatorAuth,
} = createResponseAwareAuth(rawOperatorAuth);

export {
  operatorAuth,
  uncachedOperatorAuth,
  operatorHandlers,
  withOperatorAuth,
};
