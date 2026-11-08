import { createOperatorJsonProxy } from "~/lib/operator/route-wrapper.server";

export const DELETE = createOperatorJsonProxy({
  method: "DELETE",
  path: (_request, params) =>
    `/operator/auth/passkeys/${encodeURIComponent(String(params.id))}`,
});
