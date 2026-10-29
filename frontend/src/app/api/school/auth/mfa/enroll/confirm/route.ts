import { createPortalLoginRoute } from "~/lib/portal-login-route.server";

export const POST = createPortalLoginRoute("school", "mfa/enroll/confirm");
