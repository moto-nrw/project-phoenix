import { createOperatorJsonProxy } from "~/lib/operator/route-wrapper.server";

export const POST = createOperatorJsonProxy({
  method: "POST",
  path: (_request, params) =>
    `/operator/operators/${encodeURIComponent(String(params.id))}/deactivate`,
  body: "none",
});
