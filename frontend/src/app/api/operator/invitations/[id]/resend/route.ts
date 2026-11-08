import { createOperatorJsonProxy } from "~/lib/operator/route-wrapper.server";

export const POST = createOperatorJsonProxy({
  method: "POST",
  path: (_request, params) =>
    `/operator/invitations/${encodeURIComponent(String(params.id))}/resend`,
  body: "none",
});
