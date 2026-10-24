import { proxyPut } from "~/lib/school/route-wrapper.server";
import { problemResponse } from "~/lib/api-helpers.server";

const VALID_TYPE_PATTERN = /^[a-z0-9_]{1,64}$/;

const forward = proxyPut<null, { enabled: boolean }>(
  (params) => `/school/notifications/preferences/${params.type as string}`,
);

// Eine Entscheidung setzen (#2208). Der Typ ist ein Katalogschlüssel und
// wird vor dem Weiterleiten auf seine Form geprüft.
export const PUT: typeof forward = (request, context) => {
  const type = request.nextUrl.pathname.split("/").at(-1) ?? "";
  if (!VALID_TYPE_PATTERN.test(type)) {
    return Promise.resolve(
      problemResponse(400, "invalid notification type", request),
    );
  }
  return forward(request, context);
};
