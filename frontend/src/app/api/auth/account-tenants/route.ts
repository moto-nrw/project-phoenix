import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

/** List the tenants available to the current account. */
export const GET = createTenantJsonProxy({
  method: "GET",
  path: "/auth/account/tenants",
});
