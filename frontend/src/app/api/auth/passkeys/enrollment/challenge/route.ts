import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const POST = createTenantJsonProxy({
  method: "POST",
  path: "/auth/passkeys/enrollment/challenge",
  body: "optional-json",
});
