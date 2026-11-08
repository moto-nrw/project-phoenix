import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const PUT = createTenantJsonProxy({
  method: "PUT",
  path: (_request, params) =>
    `/auth/accounts/${encodeURIComponent(String(params.accountId))}/mfa/override`,
  body: "optional-json",
});
