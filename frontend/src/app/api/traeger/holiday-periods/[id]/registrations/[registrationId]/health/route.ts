import { proxyGet } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Die Gesundheitsangaben einer Ferienanmeldung (#4722). Nur das Recht
// „Gesundheitsangaben sehen“ öffnet sie; jedes Öffnen steht im Protokoll.
export const GET = proxyGet(
  (params) =>
    `/traeger/holiday-periods/${requirePathSegmentParam(params)}/registrations/${requirePathSegmentParam(params, "registrationId")}/health`,
);
