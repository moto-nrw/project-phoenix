import { createPortalLoginRoute } from "~/lib/login-route.server";
import { operatorPortal } from "~/server/auth/operator";

export const POST = createPortalLoginRoute(
  operatorPortal,
  "passkeys/login/verify",
);
