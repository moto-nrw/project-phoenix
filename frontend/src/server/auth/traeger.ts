import "server-only";
import { createPortalAuth } from "./portal-auth";
import { PORTALS } from "./portals";

export const traegerPortal = createPortalAuth(PORTALS.traeger);
