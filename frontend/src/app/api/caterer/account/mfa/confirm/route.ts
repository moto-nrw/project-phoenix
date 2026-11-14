import { caterer } from "~/lib/caterer/route-wrapper.server";
import { createFactorConfirmRoute } from "~/lib/factor-routes.server";
import { catererPortal } from "~/server/auth/caterer";

export const POST = createFactorConfirmRoute({
  portal: catererPortal,
  handlers: caterer,
  path: "/caterer/account/mfa/confirm",
  retry: { off: "a renewed session would send the code twice" },
  log: {
    component: "CatererMFAConfirmRoute",
    event: "caterer_mfa_confirm_failed",
  },
});
