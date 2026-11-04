/**
 * School auth instance ("moto schule", #2207).
 *
 * Separate NextAuth instance with host-only cookies (no domain), so the
 * school session is invisible to tenant, operator, and parents hosts.
 *
 * Import from "~/server/auth/school" in school-app code.
 */

import "server-only";
import NextAuth from "next-auth";

import { schoolAuthConfig } from "./school-config";
import { createSessionWriter } from "./session-writer";
import { createResponseAwareAuth } from "./route-handler";

const {
  auth: rawSchoolAuth,
  handlers: schoolHandlers,
  signIn,
} = NextAuth(schoolAuthConfig);
const {
  auth: schoolAuth,
  uncachedAuth: uncachedSchoolAuth,
  withAuthResponse: withSchoolAuth,
  withSessionResponse: withSchoolSessionResponse,
  mutateSession,
} = createResponseAwareAuth(rawSchoolAuth, "school");

export { schoolAuth, uncachedSchoolAuth, schoolHandlers, withSchoolAuth };

export const setSchoolSession = createSessionWriter(
  mutateSession,
  signIn,
  "school-credentials",
);
export { withSchoolSessionResponse };
