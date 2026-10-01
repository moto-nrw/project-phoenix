import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

export const POST = createTenantJsonProxy({
  method: "POST",
  path: "/api/import/teachers/import",
  body: "form",
  networkErrorMessage: "Internal server error",
});
