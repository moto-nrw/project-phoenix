import { catererPortal } from "~/server/auth/caterer";
import { makePortalHandlers } from "../portal-handlers.server";

/** The caterer portal's route handlers (ADR 0077), on the caterer session. */
export const caterer = makePortalHandlers(catererPortal);

export const {
  proxyGet,
  proxyPost,
  jsonProxy: createCatererJsonProxy,
} = caterer;
