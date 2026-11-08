import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: "/auth/mfa/trusted-devices",
});
