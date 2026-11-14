import { createFactorConfirmRoute } from "~/lib/factor-routes.server";
import { parent } from "~/lib/parent/route-wrapper.server";
import { parentPortal } from "~/server/auth/parent";

export const POST = createFactorConfirmRoute({
  portal: parentPortal,
  handlers: parent,
  path: "/parent/me/mfa/confirm",
  retry: { off: "a renewed session would send the code twice" },
  log: {
    component: "ParentMFAConfirmRoute",
    event: "parent_mfa_confirm_failed",
  },
});
