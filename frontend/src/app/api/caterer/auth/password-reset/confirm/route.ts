import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

export const POST = createPublicJsonProxy({
  method: "POST",
  path: "/caterer/auth/password-reset/confirm",
  forwardClientHeaders: true,
});
