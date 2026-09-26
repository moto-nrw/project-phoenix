import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

// The signed preview token in the body is the credential, even after the
// browser's admin session expires. The backend validates it and writes audit.
export const POST = createPublicJsonProxy({
  method: "POST",
  path: "/auth/staff-preview/end",
  forwardClientHeaders: true,
});
