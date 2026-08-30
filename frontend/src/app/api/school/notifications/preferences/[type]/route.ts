import { NextResponse } from "next/server";
import { proxyPut } from "~/lib/school/route-wrapper.server";

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
      NextResponse.json(
        { error: "invalid notification type" },
        { status: 400 },
      ),
    );
  }
  return forward(request, context);
};
