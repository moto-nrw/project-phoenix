import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

export const POST = createPublicJsonProxy({
  method: "POST",
  path: "/school/auth/password-reset",
  forwardClientHeaders: true,
});
