import type { NextRequest } from "next/server";
import { auth, tenantPortal, withTenantAuth } from "~/server/auth";
import { sendBackend } from "./backend-client.server";
import { forwardJsonRequest, type JsonProxyOptions } from "./json-proxy.server";
import { makePortalHandlers } from "./portal-handlers.server";

export { isStringParam } from "./route-wrapper-utils.server";

/** The tenant portal's route handlers; see makePortalHandlers. */
export const tenant = makePortalHandlers(tenantPortal);

export const {
  get: createGetHandler,
  post: createPostHandler,
  put: createPutHandler,
  patch: createPatchHandler,
  del: createDeleteHandler,
  delWithBody: createDeleteWithBodyHandler,
  jsonProxy: createTenantJsonProxy,
  proxyFile,
  proxyGet,
  proxyPost,
  proxyPut,
  proxyPatch,
  proxyDelete,
} = tenant;

/**
 * Registration may carry a tenant session (admin role checks) or none; the
 * backend decides. No retry: without a session there is nothing to renew.
 */
export function createOptionalTenantJsonProxy(options: JsonProxyOptions) {
  return withTenantAuth(async (request: NextRequest, context) => {
    const token = (await auth())?.user?.token;
    return forwardJsonRequest(request, context, options, (path, call) =>
      sendBackend(path, token, { ...call, request }),
    );
  });
}
