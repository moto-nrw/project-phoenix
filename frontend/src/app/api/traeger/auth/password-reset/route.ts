import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

export const POST = createPublicJsonProxy({
  method: "POST",
  path: "/traeger/auth/password-reset",
  forwardClientHeaders: true,
});
