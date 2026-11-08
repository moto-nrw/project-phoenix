import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { parent } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { ICS_FILE } from "~/lib/file-response.server";

export const runtime = "nodejs";

/** A parent-visible appointment as an iCalendar file on the parent session. */
export const GET = withPrivateDownloadNoStore(
  parent.proxyFile(
    (params) =>
      `/parent/me/calendar/appointments/${requirePathSegmentParam(params, "appointmentId")}/ics`,
    ICS_FILE,
  ),
);
