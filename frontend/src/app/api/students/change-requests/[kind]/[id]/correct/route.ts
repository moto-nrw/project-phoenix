import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

interface CorrectBody {
  approve: boolean;
  reason: string;
  expected_version: string;
}

interface BackendEnvelope<T> {
  data: T;
}

/**
 * Proxy POST /api/students/change-requests/{kind}/{id}/correct → backend
 * (#2267). Nimmt eine bereits gefallene Entscheidung zurück und ersetzt sie.
 * Die alte Entscheidung bleibt im Verlauf stehen. Ein 409
 * `correction_unsupported` heißt, dass sich diese Art nicht zurücknehmen
 * lässt; `request_not_decided`, dass die Anfrage noch offen ist.
 */
export const POST = createPostHandler<unknown, CorrectBody>(
  async (_request, body, token, params) => {
    const kind = String(params.kind);
    const id = String(params.id);
    const response = await apiPost<BackendEnvelope<unknown>, CorrectBody>(
      `/api/students/change-requests/${encodeURIComponent(kind)}/${encodeURIComponent(id)}/correct`,
      token,
      body,
    );
    return response.data;
  },
);
