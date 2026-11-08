import "server-only";
import { createPortalAuth } from "./portal-auth";
import { PORTALS } from "./portals";

export const operatorPortal = createPortalAuth(PORTALS.operator);
