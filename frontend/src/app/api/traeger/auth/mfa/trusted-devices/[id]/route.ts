import { traeger } from "~/lib/traeger/route-wrapper.server";

export const DELETE = traeger.jsonProxy({
  method: "DELETE",
  path: (_request, params) =>
    `/traeger/auth/mfa/trusted-devices/${encodeURIComponent(String(params.id))}`,
});
