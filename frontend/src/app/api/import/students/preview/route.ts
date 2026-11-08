import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const POST = createTenantJsonProxy({
  method: "POST",
  path: "/api/import/students/preview",
  body: "form",
});
