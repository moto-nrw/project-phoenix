import { createLogoutRoute } from "~/lib/logout-route.server";
import { catererPortal } from "~/server/auth/caterer";

export const POST = createLogoutRoute(catererPortal);
