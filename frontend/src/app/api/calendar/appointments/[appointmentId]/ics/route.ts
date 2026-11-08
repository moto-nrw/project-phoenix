import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { ICS_FILE } from "~/lib/file-response.server";

export const runtime = "nodejs";

/** One appointment as an iCalendar file on the tenant session. */
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/api/calendar/appointments/${requirePathSegmentParam(params, "appointmentId")}/ics`,
    ICS_FILE,
  ),
);
