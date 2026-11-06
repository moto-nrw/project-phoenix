import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import { allowedQueryString } from "~/lib/route-wrapper-utils.server";
import {
  mapCourseWaitlists,
  type CourseWaitlist,
  type CourseWaitlistWire,
} from "~/lib/course-waitlist-helpers";

interface BackendEnvelope {
  data: CourseWaitlistWire[];
}

/**
 * Wartelisten der AGs für das Team (#4767). Mit `activity_group_id` nur die
 * eine AG, auch wenn niemand auf sie wartet. Lesbar mit users:update, wie die
 * Entscheidung über die Anfragen selbst.
 */
export const GET = createGetHandler(
  async (request, token): Promise<CourseWaitlist[]> => {
    const response = await apiGet<BackendEnvelope>(
      `/api/students/offering-change-requests/course-waitlists${allowedQueryString(request, ["activity_group_id"])}`,
      token,
    );
    return mapCourseWaitlists(response.data);
  },
);
