import { proxyUpload } from "~/lib/traeger/route-wrapper.server";

// Vorschau des Mandatsimports (#6198): liest die Datei mit der Zuordnung und
// beschreibt jede Zeile. Speichert nichts.
const IMPORT_MAX_BYTES = 2 * 1024 * 1024 + 64 * 1024;

export const POST = proxyUpload(
  "/traeger/sepa-mandates/imports/preview",
  IMPORT_MAX_BYTES,
  {
    method: "POST",
    tooLargeCode: "parent_billing.sepa_import_file_too_large",
  },
);
