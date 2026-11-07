import { createLogoutHandler } from "~/lib/logout-route.server";
import { auth, uncachedAuth, withTenantAuth } from "~/server/auth";
import { PORTALS } from "~/server/auth/portals";

export const POST = withTenantAuth(
  createLogoutHandler(PORTALS.tenant, auth, uncachedAuth),
);
