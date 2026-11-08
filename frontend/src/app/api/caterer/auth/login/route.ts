import { createPortalLoginRoute } from "~/lib/login-route.server";
import { catererPortal } from "~/server/auth/caterer";

export const POST = createPortalLoginRoute(catererPortal, "login");
