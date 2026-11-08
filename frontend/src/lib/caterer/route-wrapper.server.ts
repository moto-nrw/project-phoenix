import {
  catererAuth,
  uncachedCatererAuth,
  withCatererAuth,
} from "~/server/auth/caterer";
import { makePortalHandlers } from "../portal-handlers.server";

/** The caterer portal's route handlers (ADR 0077), on the caterer session. */
export const caterer = makePortalHandlers({
  key: "caterer",
  auth: () => catererAuth(),
  uncachedAuth: () => uncachedCatererAuth(),
  withAuth: (handler) => withCatererAuth(handler),
});

export const {
  proxyGet,
  proxyPost,
  jsonProxy: createCatererJsonProxy,
} = caterer;
