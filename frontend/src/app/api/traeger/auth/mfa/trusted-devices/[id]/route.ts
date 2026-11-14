import { traeger } from "~/lib/traeger/route-wrapper.server";

// Ein gemerktes Gerät entfernen (#6955): Dort fragt die nächste Anmeldung
// wieder nach dem Code.

export const DELETE = traeger.jsonProxy({
  method: "DELETE",
  path: (_request, params) =>
    `/traeger/auth/mfa/trusted-devices/${encodeURIComponent(String(params.id))}`,
});
