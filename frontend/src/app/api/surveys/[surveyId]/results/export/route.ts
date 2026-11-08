import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { tenantPortal } from "~/server/auth";
import { createFileDownloadRoute } from "~/lib/export-proxy.server";

/**
 * Proxy GET /api/surveys/{surveyId}/results/export → backend (#4781): the
 * Auswertung as PDF. It holds the same sums and distributions as the screen,
 * never a single answer of a person.
 */
export const GET = withPrivateDownloadNoStore(
  tenantPortal.withAuth(
    createFileDownloadRoute<{ surveyId: string }>({
      backendPath: ({ surveyId }) =>
        `/api/surveys/${encodeURIComponent(surveyId)}/results/export`,
    }),
  ),
);
