import { createCatererJsonProxy } from "~/lib/caterer/route-wrapper.server";
import { currentPasswordBody } from "~/lib/factor-routes.server";

export const POST = createCatererJsonProxy({
  method: "POST",
  path: "/caterer/account/mfa/disable",
  retry: { off: "a renewed session would send the current password twice" },
  mapJsonBody: currentPasswordBody,
});
