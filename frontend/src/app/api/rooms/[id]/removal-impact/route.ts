import { proxyGet } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// Was das Entfernen eines Raums bewirkt: löschen, archivieren oder was ihn
// noch hält (#6845).
export const GET = proxyGet(
  (params) =>
    `/api/rooms/${requirePathSegmentParam(params, "id")}/removal-impact`,
);
