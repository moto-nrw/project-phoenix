import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

/** Backend selects the new tenant; this route stays in the tenant session. */
export const POST = createTenantJsonProxy({
  method: "POST",
  path: "/auth/switch-tenant",
});
