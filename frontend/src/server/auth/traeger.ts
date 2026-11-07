import "server-only";
import { createPortalAuth } from "./portal-auth";
import { PORTALS } from "./portals";

export const {
  auth: traegerAuth,
  uncachedAuth: uncachedTraegerAuth,
  handlers: traegerHandlers,
  withAuthResponse: withTraegerAuth,
  withSessionResponse: withTraegerSessionResponse,
  setSession: setTraegerSession,
} = createPortalAuth(PORTALS.traeger);
