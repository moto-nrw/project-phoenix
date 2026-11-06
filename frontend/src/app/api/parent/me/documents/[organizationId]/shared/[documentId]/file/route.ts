import type { NextRequest } from "next/server";
import { parentDocumentPdfHandler } from "~/lib/parent-document-pdf.server";
import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { withParentAuth } from "~/server/auth/parent-route";

// Dokumentenpostfach (#4692): ein Dokument, das der Träger für Eltern
// freigegeben hat, mit dem Dateityp, den das Backend nennt.
const handler = parentDocumentPdfHandler(
  ({ organizationId, documentId }) =>
    `/parent/me/documents/${organizationId}/shared/${documentId}/file`,
);

async function GETHandler(
  request: NextRequest,
  {
    params,
  }: { params: Promise<{ organizationId: string; documentId: string }> },
) {
  const { organizationId, documentId } = await params;
  return handler(request, { organizationId, documentId });
}

export const GET = withPrivateDownloadNoStore(withParentAuth(GETHandler));
