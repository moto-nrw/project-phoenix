import { createPortalLoginRoute } from "~/lib/portal-login-route.server";

export const POST = createPortalLoginRoute("traeger", "mfa/enroll/confirm");
