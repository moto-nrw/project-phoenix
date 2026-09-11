import { proxyPost } from "~/lib/school/route-wrapper.server";

// Kenntnisnahme einer Tagesinformation durch eine Lehrkraft (#2208). Nur
// Hinweise, die moto schule erreichen, sind hier bestätigbar; das prüft das
// Backend an der Leserart der Route.
export const POST = proxyPost(
  (params) =>
    `/school/staff-notices/${encodeURIComponent(params.noticeId as string)}/acknowledge`,
);
