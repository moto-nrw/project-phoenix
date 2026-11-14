import { proxyGet, proxyUpload } from "~/lib/traeger/route-wrapper.server";

// Import bestehender Betreuungsverträge (#4723): GET listet die Importe, POST
// übernimmt die bereiten Zeilen einer CSV-Datei. Das Backend prüft Größe,
// Endung und Inhalt und speichert die Datei nicht.
// The file plus the mapping.
const IMPORT_MAX_BYTES = 2 * 1024 * 1024 + 256 * 1024;

export const GET = proxyGet("/traeger/contract-imports");

export const POST = proxyUpload("/traeger/contract-imports", IMPORT_MAX_BYTES, {
  method: "POST",
  tooLargeCode: "care_contract.import_file_too_large",
});
