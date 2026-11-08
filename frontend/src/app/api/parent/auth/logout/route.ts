import { createLogoutRoute } from "~/lib/logout-route.server";
import { parentPortal } from "~/server/auth/parent";

export const POST = createLogoutRoute(parentPortal);
