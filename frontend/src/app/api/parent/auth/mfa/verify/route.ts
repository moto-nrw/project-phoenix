import { createPortalLoginRoute } from "~/lib/login-route.server";
import { parentPortal } from "~/server/auth/parent";

// The second step of a parent sign-in with the code (#7051). The BFF installs
// the session; the backend's trusted-device cookie passes through on
// "remember_device".
export const POST = createPortalLoginRoute(parentPortal, "mfa/verify");
