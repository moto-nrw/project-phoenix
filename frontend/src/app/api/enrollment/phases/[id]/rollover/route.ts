import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const POST = createTenantJsonProxy({
  method: "POST",
  path: (_request, params) =>
    `/api/enrollment/phases/${encodeURIComponent(String(params.id))}/rollover`,
});
