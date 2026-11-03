import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyUpload } from "~/lib/traeger/route-wrapper.server";

// Nachweis, den das Träger-Büro für die Familie hochlädt (#5627). Das Backend
// prüft Art, Größe und Inhalt wie beim Nachweis der Eltern (#5238).
/** HolidayProofOptions: at most 10 MB per proof (#5238). */
const PROOF_MAX_MB = 10;

export const POST = proxyUpload(
  (params) =>
    `/traeger/holiday-periods/${requirePathSegmentParam(params)}/registrations/${requirePathSegmentParam(params, "registrationId")}/proofs`,
  PROOF_MAX_MB * 1024 * 1024,
  {
    method: "POST",
    tooLargeCode: "enrollment.upload_invalid",
    // The catalog text names the limit, like the backend's own refusal.
    tooLargeDetails: { max_mb: PROOF_MAX_MB },
  },
);
