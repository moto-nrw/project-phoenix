import {
  createParentDeleteHandler,
  parentApiDelete,
} from "~/lib/parent/route-wrapper.server";

interface BackendPickupChangeRequest {
  id: string;
  date: string;
  pickup_time: string;
  reason: string;
  status: string;
  decision_reason?: string;
  created_at: string;
  reviewed_at?: string;
}

export const DELETE = createParentDeleteHandler<BackendPickupChangeRequest>(
  async (_request, token, params) => {
    const studentId = String(params.studentId);
    const requestId = String(params.requestId);
    return parentApiDelete<BackendPickupChangeRequest>(
      `/parent/me/children/${encodeURIComponent(studentId)}/pickup-change-requests/${encodeURIComponent(requestId)}`,
      token,
    );
  },
);
