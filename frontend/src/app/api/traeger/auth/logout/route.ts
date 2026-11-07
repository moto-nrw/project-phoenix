import { createLogoutHandler } from "~/lib/logout-route.server";
import {
  traegerAuth,
  uncachedTraegerAuth,
  withTraegerAuth,
} from "~/server/auth/traeger";
import { PORTALS } from "~/server/auth/portals";

export const POST = withTraegerAuth(
  createLogoutHandler(PORTALS.traeger, traegerAuth, uncachedTraegerAuth),
);
