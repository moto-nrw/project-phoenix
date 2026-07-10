import {
  createParentDeleteHandler,
  parentApiDelete,
} from "~/lib/parent/route-wrapper.server";

interface BackendExcusedRequest {
  id: string;
  student_id: string;
  status: string;
  dates: string[];
  note: string;
  decision_reason?: string;
  created_at: string;
  reviewed_at?: string;
  is_self: boolean;
}

/**
 * DELETE /api/parent/me/children/{studentId}/excused-requests/{requestId} →
 * backend. Withdraws the guardian's own still-pending excused request and
 * returns the updated request (now `withdrawn`). The backend rejects a withdraw
 * once the OGS has decided the request, and verifies guardianship from the JWT.
 */
export const DELETE = createParentDeleteHandler<BackendExcusedRequest>(
  async (_request, token, params) => {
    const studentId = String(params.studentId);
    const requestId = String(params.requestId);
    return parentApiDelete<BackendExcusedRequest>(
      `/parent/me/children/${encodeURIComponent(studentId)}/excused-requests/${encodeURIComponent(requestId)}`,
      token,
    );
  },
);
