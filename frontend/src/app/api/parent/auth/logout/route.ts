import { createLogoutHandler } from "~/lib/logout-route.server";
import {
  parentAuth,
  uncachedParentAuth,
  withParentAuth,
} from "~/server/auth/parent";
import { PORTALS } from "~/server/auth/portals";

export const POST = withParentAuth(
  createLogoutHandler(PORTALS.parent, parentAuth, uncachedParentAuth),
);
