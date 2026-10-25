/**
 * Träger auth instance ("moto träger", #4649).
 *
 * Separate NextAuth instance with host-only cookies (no domain), so the
 * Träger session is invisible to tenant, operator, parents, and school hosts.
 *
 * Import from "~/server/auth/traeger" in Träger-portal code.
 */

import NextAuth from "next-auth";

import { traegerAuthConfig } from "./traeger-config";
import { createResponseAwareAuth } from "./route-handler";

const { auth: rawTraegerAuth, handlers: traegerHandlers } =
  NextAuth(traegerAuthConfig);
const {
  auth: traegerAuth,
  uncachedAuth: uncachedTraegerAuth,
  withAuthResponse: withTraegerAuth,
} = createResponseAwareAuth(rawTraegerAuth, "traeger");

export { traegerAuth, uncachedTraegerAuth, traegerHandlers, withTraegerAuth };
