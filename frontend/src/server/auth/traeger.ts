import "server-only";
import { createPortalAuth } from "./portal-auth";
import { PORTALS } from "./portals";

/** The traeger portal; see createPortalAuth. */
export const traegerPortal = createPortalAuth(PORTALS.traeger);

export const {
  auth: traegerAuth,
  handlers: traegerHandlers,
  withAuthResponse: withTraegerAuth,
} = traegerPortal;
