import "server-only";
import { createPortalAuth } from "./portal-auth";
import { PORTALS } from "./portals";

/** The operator portal; see createPortalAuth. */
export const operatorPortal = createPortalAuth(PORTALS.operator);

export const {
  auth: operatorAuth,
  handlers: operatorHandlers,
  withAuthResponse: withOperatorAuth,
} = operatorPortal;
