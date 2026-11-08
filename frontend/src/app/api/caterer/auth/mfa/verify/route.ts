import { createPortalLoginRoute } from "~/lib/login-route.server";
import { catererPortal } from "~/server/auth/caterer";

// The second step of a caterer login with two-factor sign-in (#4497). The
// BFF installs the session; the backend's trusted-device cookie passes
// through on "remember_device".
export const POST = createPortalLoginRoute(catererPortal, "mfa/verify");
