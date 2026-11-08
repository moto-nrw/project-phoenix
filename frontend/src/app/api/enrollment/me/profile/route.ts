import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

/** Require a session so the enrollment form can omit unavailable autofill. */
export const GET = createTenantJsonProxy({
  method: "GET",
  path: "/api/enrollment/me/profile",
});
