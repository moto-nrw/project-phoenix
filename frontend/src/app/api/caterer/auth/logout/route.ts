import { createLogoutHandler } from "~/lib/logout-route.server";
import {
  catererAuth,
  uncachedCatererAuth,
  withCatererAuth,
} from "~/server/auth/caterer";
import { PORTALS } from "~/server/auth/portals";

export const POST = withCatererAuth(
  createLogoutHandler(PORTALS.caterer, catererAuth, uncachedCatererAuth),
);
