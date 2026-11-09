import { proxyGet, proxyUpload } from "~/lib/traeger/route-wrapper.server";

// Import bestehender SEPA-Mandate (#6198): GET listet die Importe, POST
// übernimmt die bereiten Zeilen einer CSV-Datei. Das Backend prüft Größe,
// Endung und Inhalt und speichert die Datei nicht.
const IMPORT_MAX_BYTES = 2 * 1024 * 1024 + 64 * 1024;

export const GET = proxyGet("/traeger/sepa-mandates/imports");

export const POST = proxyUpload(
  "/traeger/sepa-mandates/imports",
  IMPORT_MAX_BYTES,
  {
    method: "POST",
    tooLargeCode: "parent_billing.sepa_import_file_too_large",
  },
);
