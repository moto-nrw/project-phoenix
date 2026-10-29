import { createPortalLoginRoute } from "~/lib/portal-login-route.server";

export const POST = createPortalLoginRoute("operator", "mfa/enroll/confirm");
