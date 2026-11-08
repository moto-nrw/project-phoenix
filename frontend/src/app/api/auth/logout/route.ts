import { createLogoutRoute } from "~/lib/logout-route.server";
import { tenantPortal } from "~/server/auth";

export const POST = createLogoutRoute(tenantPortal);
