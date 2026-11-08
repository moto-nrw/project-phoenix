// Dokumentenpostfach (#4671, #4678): ein Dokument des Trägers als
// gespeichertes PDF, ein Schreiben oder ein Betreuungsvertrag. Ein Dokument
// außerhalb des Postfachs beantwortet das Backend mit 404, ein Konto ohne
// Recht mit 403.

import type { NextRequest } from "next/server";
import { handleApiError, problemResponse } from "./api-response-error.server";
import { parent } from "./parent/route-wrapper.server";

/** Die Teile des Pfads: der Träger und das Dokument, beide Zahlen. */
interface DocumentKey {
  readonly organizationId: string;
  readonly documentId: string;
}

/**
 * Baut den Handler für ein PDF des Postfachs; `backendPath` nennt den
 * Backend-Pfad des Dokuments aus den geprüften Teilen.
 */
export function parentDocumentPdfHandler(
  backendPath: (key: DocumentKey) => string,
) {
  return async (request: NextRequest, key: DocumentKey) => {
    if (!/^\d+$/.test(key.organizationId) || !/^\d+$/.test(key.documentId)) {
      return problemResponse(400, "invalid document", request);
    }
    try {
      return await parent.downloadFile(request, backendPath(key), {
        backendCacheControl: true,
      });
    } catch (error) {
      return handleApiError(error, request);
    }
  };
}
