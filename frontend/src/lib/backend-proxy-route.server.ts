import type { NextRequest } from "next/server";
import { withPrivateNoStore } from "~/lib/private-no-store";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";
import { sendBackend } from "./backend-client.server";
import { forwardJsonRequest, type JsonProxyOptions } from "./json-proxy.server";

export type { JsonProxyOptions } from "./json-proxy.server";

/** Public JSON proxy; no session and no Authorization header travel. */
export function createPublicJsonProxy(options: JsonProxyOptions) {
  // No auth handler applies the cache policy here, and token routes
  // (enrollment edit links) answer with personal data.
  return withPrivateNoStore((request: NextRequest, context?: RouteContext) =>
    forwardJsonRequest(request, context, options, (path, call) =>
      sendBackend(path, null, { ...call, request }),
    ),
  );
}
