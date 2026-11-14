import { createParentJsonProxy } from "~/lib/parent/route-wrapper.server";
import { currentPasswordBody } from "~/lib/factor-routes.server";

export const POST = createParentJsonProxy({
  method: "POST",
  path: "/parent/me/mfa/disable",
  retry: { off: "a renewed session would send the current password twice" },
  mapJsonBody: currentPasswordBody,
});
