import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const DELETE = createTenantJsonProxy({
  method: "DELETE",
  path: (_request, params) =>
    `/auth/mfa/trusted-devices/${encodeURIComponent(String(params.id))}`,
});
