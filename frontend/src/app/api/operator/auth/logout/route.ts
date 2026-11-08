import { createLogoutRoute } from "~/lib/logout-route.server";
import { operatorPortal } from "~/server/auth/operator";

export const POST = createLogoutRoute(operatorPortal);
