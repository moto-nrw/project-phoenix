/**
 * Operator auth instance.
 *
 * Separate NextAuth instance with host-only cookies (no domain),
 * so the operator session is invisible to tenant subdomains.
 *
 * Import from "~/server/auth/operator" in operator-specific code.
 */

import NextAuth from "next-auth";
import { cache } from "react";

import { operatorAuthConfig } from "./operator-config";

const {
  auth: uncachedOperatorAuth,
  handlers: operatorHandlers,
  signIn: operatorSignIn,
} = NextAuth(operatorAuthConfig);

const operatorAuth = cache(uncachedOperatorAuth);

export { operatorAuth, uncachedOperatorAuth, operatorHandlers, operatorSignIn };
