import { proxyDelete, proxyPut } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Ein Zeitfenster am Elternsprechtag buchen, verschieben oder absagen
// (#4801). Das Recht dazu prüft das Backend je Kind.
const endpoint = (params: Record<string, unknown>) =>
  `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/parent-conferences/${requirePathSegmentParam(params, "conferenceId")}/booking`;

export const PUT = proxyPut(endpoint);
export const DELETE = proxyDelete(endpoint);
