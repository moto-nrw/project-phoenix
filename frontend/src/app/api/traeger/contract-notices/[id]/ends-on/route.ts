import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPut } from "~/lib/traeger/route-wrapper.server";

// Vertragsende einer außerordentlichen Kündigung (#4682): das Träger-Büro
// legt es fest, nachdem es den Grund der Eltern geprüft hat.
export const PUT = proxyPut(
  (params) =>
    `/traeger/contract-notices/${requirePathSegmentParam(params)}/ends-on`,
);
