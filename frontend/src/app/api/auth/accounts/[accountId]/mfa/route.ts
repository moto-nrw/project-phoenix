import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: (_request, params) =>
    `/auth/accounts/${encodeURIComponent(String(params.accountId))}/mfa`,
});

export const DELETE = createTenantJsonProxy({
  method: "DELETE",
  path: (_request, params) =>
    `/auth/accounts/${encodeURIComponent(String(params.accountId))}/mfa`,
  body: "optional-json",
});
