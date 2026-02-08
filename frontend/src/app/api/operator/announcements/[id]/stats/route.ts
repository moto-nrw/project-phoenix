import type { AnnouncementStats } from "~/lib/operator/announcements-helpers";
import {
  createOperatorGetHandler,
  operatorApiGet,
  isStringParam,
} from "~/lib/operator/route-wrapper";

export const GET = createOperatorGetHandler<AnnouncementStats>(
  async (_request, token, params) => {
    const id = params.id;
    if (!isStringParam(id)) {
      throw new Error("Invalid announcement ID");
    }
    return operatorApiGet<AnnouncementStats>(
      `/operator/announcements/${id}/stats`,
      token,
    );
  },
);
