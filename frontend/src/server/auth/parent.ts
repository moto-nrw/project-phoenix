import "server-only";
import { createPortalAuth } from "./portal-auth";
import { PORTALS } from "./portals";

export const {
  auth: parentAuth,
  uncachedAuth: uncachedParentAuth,
  handlers: parentHandlers,
  withAuthResponse: withParentAuth,
  withSessionResponse: withParentSessionResponse,
  setSession: setParentSession,
} = createPortalAuth(PORTALS.parent);
