import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { operatorJsonErrors } from "~/lib/operator/route-wrapper.server";

export const POST = createPublicJsonProxy({
  method: "POST",
  path: "/operator/auth/email-confirm",
  ...operatorJsonErrors,
});
