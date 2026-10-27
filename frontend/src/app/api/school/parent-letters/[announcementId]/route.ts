import {
  proxyDelete,
  proxyGet,
  proxyPut,
} from "~/lib/school/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Ein eigener Elternbrief in moto schule (#4800): lesen, den Entwurf ändern
// oder löschen.
export const GET = proxyGet(
  (p) =>
    `/school/parent-letters/${requirePathSegmentParam(p, "announcementId")}`,
);
export const PUT = proxyPut(
  (p) =>
    `/school/parent-letters/${requirePathSegmentParam(p, "announcementId")}`,
);
export const DELETE = proxyDelete(
  (p) =>
    `/school/parent-letters/${requirePathSegmentParam(p, "announcementId")}`,
);
