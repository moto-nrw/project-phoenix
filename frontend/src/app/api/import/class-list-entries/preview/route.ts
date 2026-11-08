import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const POST = createTenantJsonProxy({
  method: "POST",
  path: "/api/import/class-list-entries/preview",
  body: "form",
});
