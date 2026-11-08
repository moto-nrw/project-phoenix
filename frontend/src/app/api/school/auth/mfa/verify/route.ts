import { createPortalLoginRoute } from "~/lib/login-route.server";
import { schoolPortal } from "~/server/auth/school";

export const POST = createPortalLoginRoute(schoolPortal, "mfa/verify");
