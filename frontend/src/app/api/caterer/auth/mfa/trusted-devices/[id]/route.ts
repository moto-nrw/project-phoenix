import { createCatererJsonProxy } from "~/lib/caterer/route-wrapper.server";

export const DELETE = createCatererJsonProxy({
  method: "DELETE",
  path: (_request, params) =>
    `/caterer/auth/mfa/trusted-devices/${encodeURIComponent(String(params.id))}`,
  forwardClientHeaders: true,
});
