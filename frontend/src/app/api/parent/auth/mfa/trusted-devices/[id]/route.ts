import { createParentJsonProxy } from "~/lib/parent/route-wrapper.server";

export const DELETE = createParentJsonProxy({
  method: "DELETE",
  path: (_request, params) =>
    `/parent/me/mfa/trusted-devices/${encodeURIComponent(String(params.id))}`,
});
