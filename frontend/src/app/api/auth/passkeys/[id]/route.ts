import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const DELETE = createTenantJsonProxy({
  method: "DELETE",
  path: (_request, params) =>
    `/auth/passkeys/${encodeURIComponent(String(params.id))}`,
});
