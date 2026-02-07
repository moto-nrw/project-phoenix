import type { BackendAnnouncementViewDetail } from "~/lib/operator/announcements-helpers";
import {
  createOperatorGetHandler,
  operatorApiGet,
  isStringParam,
} from "~/lib/operator/route-wrapper";

export const GET = createOperatorGetHandler<BackendAnnouncementViewDetail[]>(
  async (_request, token, params) => {
    const id = params.id;
    if (!isStringParam(id)) {
      throw new Error("Invalid announcement ID");
    }
    return operatorApiGet<BackendAnnouncementViewDetail[]>(
      `/operator/announcements/${id}/views`,
      token,
    );
  },
);
