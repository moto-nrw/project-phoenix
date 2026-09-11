import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

interface MarkDoneBody {
  expected_version: string;
  reason?: string;
}

interface BackendEnvelope<T> {
  data: T;
}

/**
 * Proxy POST /api/students/change-requests/{kind}/{id}/mark-done → backend
 * (#2267). Schließt eine Anfrage ab, die nur noch vergangene Tage betrifft:
 * nichts wird übernommen, die Zeile verlässt nur die Arbeitsliste. Ein 409
 * `request_not_past` heißt, dass die Anfrage noch kommende Tage betrifft und
 * regulär entschieden werden muss; `change_request_stale` heißt, dass sie
 * inzwischen geändert wurde.
 */
export const POST = createPostHandler<unknown, MarkDoneBody>(
  async (_request, body, token, params) => {
    const kind = String(params.kind);
    const id = String(params.id);
    const response = await apiPost<BackendEnvelope<unknown>, MarkDoneBody>(
      `/api/students/change-requests/${encodeURIComponent(kind)}/${encodeURIComponent(id)}/mark-done`,
      token,
      body,
    );
    return response.data;
  },
);
