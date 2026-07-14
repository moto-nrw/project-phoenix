import type { NextRequest } from "next/server";
import { apiDelete } from "~/lib/api-helpers.server";
import { createDeleteHandler } from "~/lib/route-wrapper.server";

/**
 * DELETE /api/staff/[id]/absences/[absenceId]
 * Admin endpoint: delete a staff member's absence. Deleting a `sick`
 * absence reverses the Dienst-/Betreuungsplan cascade backend-side (#1843).
 */
export const DELETE = createDeleteHandler<null>(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const absenceId = params.absenceId as string;
    await apiDelete(`/api/staff/${id}/absences/${absenceId}`, token);
    return null;
  },
);
