import { createPortalLoginRoute } from "~/lib/portal-login-route.server";

export const POST = createPortalLoginRoute("operator", "passkeys/login/verify");
