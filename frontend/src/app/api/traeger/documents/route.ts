import { proxyGet, proxyUpload } from "~/lib/traeger/route-wrapper.server";
import { TRAEGER_DOCUMENT_MAX_BYTES } from "~/lib/traeger-documents-api";

// Dokumente, die der Träger mit Eltern und Schulen teilt (#4692): die Liste
// und das Hochladen einer Datei mit Titel. Typ, Größe und Inhalt prüft das
// Backend.
export const GET = proxyGet("/traeger/documents/");

export const POST = proxyUpload(
  "/traeger/documents/",
  TRAEGER_DOCUMENT_MAX_BYTES,
  {
    method: "POST",
    tooLargeCode: "files.file_too_large",
    tooLargeDetails: { max_mb: TRAEGER_DOCUMENT_MAX_BYTES / (1024 * 1024) },
  },
);
