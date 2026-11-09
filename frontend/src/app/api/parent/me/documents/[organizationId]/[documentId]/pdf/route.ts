import type { NextRequest } from "next/server";
import { parentDocumentPdfHandler } from "~/lib/parent-document-pdf.server";
import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { parentPortal } from "~/server/auth/parent";

// Dokumentenpostfach (#4671): ein Schreiben des Trägers als gespeichertes PDF.
const handler = parentDocumentPdfHandler(
  ({ organizationId, documentId }) =>
    `/parent/me/documents/${organizationId}/${documentId}/pdf`,
);

async function GETHandler(
  request: NextRequest,
  {
    params,
  }: { params: Promise<{ organizationId: string; documentId: string }> },
) {
  return handler(request, await params);
}

export const GET = withPrivateDownloadNoStore(
  parentPortal.withAuth(GETHandler),
);
