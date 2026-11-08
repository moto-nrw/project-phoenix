import "server-only";
import { createPortalAuth } from "./portal-auth";
import { PORTALS } from "./portals";

/** The caterer portal; see createPortalAuth. */
export const catererPortal = createPortalAuth(PORTALS.caterer);

export const {
  auth: catererAuth,
  handlers: catererHandlers,
  withAuthResponse: withCatererAuth,
} = catererPortal;
