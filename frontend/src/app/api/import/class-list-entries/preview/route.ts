import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

export const POST = createTenantJsonProxy({
  method: "POST",
  path: "/api/import/class-list-entries/preview",
  body: "form",
  networkErrorMessage: "Internal server error",
});
