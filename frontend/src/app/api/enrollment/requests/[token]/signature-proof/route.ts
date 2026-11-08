import type { NextRequest } from "next/server";
import { handleApiError } from "~/lib/api-response-error.server";
import { sendBackend } from "~/lib/backend-client.server";
import { fileResponse } from "~/lib/file-response.server";
import { withPrivateDownloadNoStore } from "~/lib/private-no-store";

// Public proxy for the parent's proof of a signed enrollment (#4786). The
// status token is the only authentication; the backend resolves it to one
// request of one school and answers 404 for an unknown, expired or altered
// token.
async function GETHandler(
  request: NextRequest,
  { params }: { params: Promise<{ token: string }> },
) {
  const { token } = await params;
  try {
    return await fileResponse(
      await sendBackend(
        `/api/enrollment/requests/${encodeURIComponent(token)}?format=pdf`,
        null,
        { request },
      ),
      request,
      { backendCacheControl: true },
    );
  } catch (error) {
    return handleApiError(error, request);
  }
}

export const GET = withPrivateDownloadNoStore(GETHandler);
