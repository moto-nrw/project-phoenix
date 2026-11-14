import { proxyUpload } from "~/lib/traeger/route-wrapper.server";

// Vorschau des Vertragsimports (#4723): liest die Datei mit der Zuordnung und
// beschreibt jede Zeile. Speichert nichts.
// The file plus the mapping.
const IMPORT_MAX_BYTES = 2 * 1024 * 1024 + 256 * 1024;

export const POST = proxyUpload(
  "/traeger/contract-imports/preview",
  IMPORT_MAX_BYTES,
  {
    method: "POST",
    tooLargeCode: "care_contract.import_file_too_large",
  },
);
