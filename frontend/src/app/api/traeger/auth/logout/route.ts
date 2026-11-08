import { createLogoutRoute } from "~/lib/logout-route.server";
import { traegerPortal } from "~/server/auth/traeger";

export const POST = createLogoutRoute(traegerPortal);
