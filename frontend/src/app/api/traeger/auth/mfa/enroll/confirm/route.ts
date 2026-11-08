import { createPortalLoginRoute } from "~/lib/login-route.server";
import { traegerPortal } from "~/server/auth/traeger";

export const POST = createPortalLoginRoute(traegerPortal, "mfa/enroll/confirm");
