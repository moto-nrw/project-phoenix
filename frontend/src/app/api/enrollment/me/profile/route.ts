import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

/** Require a session so the enrollment form can omit unavailable autofill. */
export const GET = createTenantJsonProxy({
  method: "GET",
  path: "/api/enrollment/me/profile",
  cache: "no-store",
  contentTypeOnGet: false,
  unauthorizedError: "Unauthenticated",
});
