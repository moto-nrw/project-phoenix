import { createPortalLoginRoute } from "~/lib/login-route.server";
import { parentPortal } from "~/server/auth/parent";

// The parents' password sign-in (#7051). Where the account has the second
// factor, the answer is the challenge of the mailed code; otherwise the BFF
// installs the session. The browser's remember-device cookie travels along.
export const POST = createPortalLoginRoute(parentPortal, "login");
