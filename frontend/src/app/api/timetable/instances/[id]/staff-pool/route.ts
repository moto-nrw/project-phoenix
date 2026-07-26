// GET /api/timetable/instances/[id]/staff-pool — Personalpool für das
// Zeitfenster eines Blocks (#1884): wer ist laut Dienstplan verfügbar, wer
// bereits einem überlappenden Block zugeordnet, wer abwesend.
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (params) =>
    `/api/timetable/instances/${requirePathSegmentParam(params)}/staff-pool`,
);
