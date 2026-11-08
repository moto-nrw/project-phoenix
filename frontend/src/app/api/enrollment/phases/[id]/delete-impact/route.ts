import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: (_request, params) =>
    `/api/enrollment/phases/${encodeURIComponent(String(params.id))}/delete-impact`,
});
