import "server-only";
import { createPortalAuth } from "./portal-auth";
import { PORTALS } from "./portals";

export const {
  auth: operatorAuth,
  uncachedAuth: uncachedOperatorAuth,
  handlers: operatorHandlers,
  withAuthResponse: withOperatorAuth,
  withSessionResponse: withOperatorSessionResponse,
  setSession: setOperatorSession,
} = createPortalAuth(PORTALS.operator);
