import { createOperatorJsonProxy } from "~/lib/operator/route-wrapper.server";

export const DELETE = createOperatorJsonProxy({
  method: "DELETE",
  path: (_request, params) =>
    `/operator/invitations/${encodeURIComponent(String(params.id))}`,
  body: "none",
});
