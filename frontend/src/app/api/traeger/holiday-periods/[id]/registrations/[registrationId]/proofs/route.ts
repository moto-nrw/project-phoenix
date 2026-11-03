import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyUpload } from "~/lib/traeger/route-wrapper.server";

// Nachweis, den das Träger-Büro für die Familie hochlädt (#5627). Das Backend
// prüft Art, Größe und Inhalt wie beim Nachweis der Eltern (#5238).
export const POST = proxyUpload(
  (params) =>
    `/traeger/holiday-periods/${requirePathSegmentParam(params)}/registrations/${requirePathSegmentParam(params, "registrationId")}/proofs`,
  10 * 1024 * 1024,
  { method: "POST", tooLargeCode: "enrollment.upload_invalid" },
);
