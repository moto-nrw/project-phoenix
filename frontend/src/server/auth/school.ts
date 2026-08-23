/**
 * School auth instance ("moto schule", #2207).
 *
 * Separate NextAuth instance with host-only cookies (no domain), so the
 * school session is invisible to tenant, operator, and parents hosts.
 *
 * Import from "~/server/auth/school" in school-app code.
 */

import NextAuth from "next-auth";

import { schoolAuthConfig } from "./school-config";
import { createResponseAwareAuth } from "./route-handler";

const { auth: rawSchoolAuth, handlers: schoolHandlers } =
  NextAuth(schoolAuthConfig);
const {
  auth: schoolAuth,
  uncachedAuth: uncachedSchoolAuth,
  withAuthResponse: withSchoolAuth,
} = createResponseAwareAuth(rawSchoolAuth);

export { schoolAuth, uncachedSchoolAuth, schoolHandlers, withSchoolAuth };
