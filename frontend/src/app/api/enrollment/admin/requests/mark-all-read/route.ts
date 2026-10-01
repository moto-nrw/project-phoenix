import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

/** Alle ungelesenen Anmeldungen der Person als gelesen markieren (#3778). */
export const POST = createTenantJsonProxy({
  method: "POST",
  path: "/api/enrollment/admin/requests/mark-all-read",
  body: "none",
  unauthorizedError: "Unauthenticated",
});
