import "server-only";
import { createPortalAuth } from "./portal-auth";
import { PORTALS } from "./portals";

/** The parent portal; see createPortalAuth. */
export const parentPortal = createPortalAuth(PORTALS.parent);

export const {
  auth: parentAuth,
  handlers: parentHandlers,
  withAuthResponse: withParentAuth,
} = parentPortal;
