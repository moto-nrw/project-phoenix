import "server-only";
import { createPortalAuth } from "./portal-auth";
import { PORTALS } from "./portals";

export const {
  auth: catererAuth,
  uncachedAuth: uncachedCatererAuth,
  handlers: catererHandlers,
  withAuthResponse: withCatererAuth,
  withSessionResponse: withCatererSessionResponse,
  setSession: setCatererSession,
} = createPortalAuth(PORTALS.caterer);
