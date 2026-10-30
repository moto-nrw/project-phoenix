// Öffentlicher Download der Monatsdatei für das Lohnbüro (#4805). Der Link
// aus der E-Mail ist die einzige Berechtigung: kein Konto, keine Sitzung.
// Nur POST öffnet ihn, denn das Backend gibt die Datei genau einmal heraus
// und ein Mailscanner, der jedem Link per GET folgt, darf ihn nicht
// verbrauchen. Bytes und Download-Kopfzeilen gehen unverändert durch.

import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";
import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { isPayrollDownloadLink } from "~/lib/payroll-download-link";

export const POST = withPrivateDownloadNoStore(
  createPublicJsonProxy({
    method: "POST",
    body: "none",
    cache: "no-store",
    onSuccess: (response) => {
      const headers = new Headers({
        "Cache-Control": "private, no-store",
        "X-Content-Type-Options": "nosniff",
      });
      for (const name of [
        "Content-Type",
        "Content-Disposition",
        "Content-Length",
      ]) {
        const value = response.headers.get(name);
        if (value) headers.set(name, value);
      }
      return new Response(response.body, { status: response.status, headers });
    },
    path: (request, params) => {
      const link = params.link;
      // Ein verstümmelter Link bekommt dieselbe Antwort wie ein verbrauchter,
      // ohne das Backend zu fragen: Die Antwort hilft nicht beim Raten.
      if (typeof link !== "string" || !isPayrollDownloadLink(link)) {
        return problemResponse(
          404,
          "payroll download link is not available",
          { code: "workforce.payroll_download_unavailable" },
          request,
        );
      }
      return `/public/payroll-download/${encodeURIComponent(link)}`;
    },
  }),
);
