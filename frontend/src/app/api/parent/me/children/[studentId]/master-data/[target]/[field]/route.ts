import {
  createParentPatchHandler,
  parentApiPatch,
} from "~/lib/parent/route-wrapper.server";

interface UpdateBody {
  value: unknown;
}

/**
 * Proxy PATCH /api/parent/me/children/{studentId}/master-data/{target}/{field}
 * → backend. Applies a Track A direct edit (auto-saved) to one field.
 */
export const PATCH = createParentPatchHandler<unknown, UpdateBody>(
  async (_request, body, token, params) => {
    const studentId = String(params.studentId);
    const target = String(params.target);
    const field = String(params.field);
    return parentApiPatch<unknown, UpdateBody>(
      `/parent/me/children/${encodeURIComponent(studentId)}/master-data/${encodeURIComponent(target)}/${encodeURIComponent(field)}`,
      token,
      body,
    );
  },
);
