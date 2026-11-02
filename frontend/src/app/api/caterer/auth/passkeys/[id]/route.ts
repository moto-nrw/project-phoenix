import { createCatererJsonProxy } from "~/lib/caterer/route-wrapper.server";

export const DELETE = createCatererJsonProxy({
  method: "DELETE",
  path: (_request, params) =>
    `/caterer/auth/passkeys/${encodeURIComponent(String(params.id))}`,
  forwardClientHeaders: true,
});
