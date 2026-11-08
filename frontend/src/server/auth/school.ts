import "server-only";
import { createPortalAuth } from "./portal-auth";
import { PORTALS } from "./portals";

export const schoolPortal = createPortalAuth(PORTALS.school);
