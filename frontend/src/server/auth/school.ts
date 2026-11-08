import "server-only";
import { createPortalAuth } from "./portal-auth";
import { PORTALS } from "./portals";

/** The school portal; see createPortalAuth. */
export const schoolPortal = createPortalAuth(PORTALS.school);

export const {
  auth: schoolAuth,
  handlers: schoolHandlers,
  withAuthResponse: withSchoolAuth,
} = schoolPortal;
