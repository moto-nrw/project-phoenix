import { createPortalLoginRoute } from "~/lib/login-route.server";
import { tenantPortal } from "~/server/auth";

export const POST = createPortalLoginRoute(tenantPortal, "mfa/verify");
