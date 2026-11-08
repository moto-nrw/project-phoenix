import type { NextRequest } from "next/server";
import { parentDocumentPdfHandler } from "~/lib/parent-document-pdf.server";
import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { parentPortal } from "~/server/auth/parent";

// Dokumentenpostfach (#4678): ein Betreuungsvertrag des Trägers als
// gespeichertes PDF.
const handler = parentDocumentPdfHandler(
  ({ organizationId, documentId }) =>
    `/parent/me/documents/${organizationId}/contracts/${documentId}/pdf`,
);

async function GETHandler(
  request: NextRequest,
  {
    params,
  }: { params: Promise<{ organizationId: string; contractId: string }> },
) {
  const { organizationId, contractId } = await params;
  return handler(request, { organizationId, documentId: contractId });
}

export const GET = withPrivateDownloadNoStore(
  parentPortal.withAuth(GETHandler),
);
