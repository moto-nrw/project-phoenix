import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const POST = createTenantJsonProxy({
  method: "POST",
  path: "/auth/passkeys/register/verify",
  body: "optional-json",
});
