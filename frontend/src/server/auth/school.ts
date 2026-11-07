import "server-only";
import { createPortalAuth } from "./portal-auth";
import { PORTALS } from "./portals";

export const {
  auth: schoolAuth,
  uncachedAuth: uncachedSchoolAuth,
  handlers: schoolHandlers,
  withAuthResponse: withSchoolAuth,
  withSessionResponse: withSchoolSessionResponse,
  setSession: setSchoolSession,
} = createPortalAuth(PORTALS.school);
