import { createParentJsonProxy } from "~/lib/parent/route-wrapper.server";

export const POST = createParentJsonProxy({
  method: "POST",
  path: "/parent/me/mfa/start",
});
