import { createLogoutRoute } from "~/lib/logout-route.server";
import { schoolPortal } from "~/server/auth/school";

export const POST = createLogoutRoute(schoolPortal);
