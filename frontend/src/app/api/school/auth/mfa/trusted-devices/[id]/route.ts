import { school } from "~/lib/school/route-wrapper.server";

export const DELETE = school.jsonProxy({
  method: "DELETE",
  path: (_request, params) =>
    `/school/auth/mfa/trusted-devices/${encodeURIComponent(String(params.id))}`,
});
