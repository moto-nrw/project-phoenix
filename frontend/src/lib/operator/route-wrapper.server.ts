import type { NextRequest } from "next/server";
import type { NextResponse } from "next/server";
import { operatorPortal } from "~/server/auth/operator";
import type { ErrorCode } from "~/lib/error-codes.generated";
import { problemResponse } from "../api-response-error.server";
import { unwrapEnvelope } from "../backend-client.server";
import type { JsonProxyOptions } from "../json-proxy.server";
import { makePortalHandlers } from "../portal-handlers.server";

export { isStringParam } from "../route-wrapper-utils.server";

/**
 * An error the operator BFF answers itself (#2519): a registered code for the
 * client and an English diagnostic, never a UI sentence. The client shows the
 * catalog text of the code.
 */
export function operatorErrorResponse(
  status: number,
  code: ErrorCode,
  diagnostic: string,
  request?: Request,
): NextResponse {
  return problemResponse(status, diagnostic, { code }, request);
}

/** The operator portal's route handlers, on the operator session only. */
export const operator = makePortalHandlers(operatorPortal);

export const {
  proxyGet,
  proxyPost,
  proxyPut,
  proxyDelete,
  get: createOperatorGetHandler,
  post: createOperatorPostHandler,
  put: createOperatorPutHandler,
  del: createOperatorDeleteHandler,
  jsonProxy: createOperatorJsonProxy,
} = operator;

/** The operator's code for a body it cannot read. */
export const operatorJsonErrors = {
  invalidJsonResponse: (request: NextRequest) =>
    operatorErrorResponse(
      400,
      "general.input",
      "Invalid JSON request body",
      request,
    ),
} satisfies Partial<JsonProxyOptions>;

export const operatorApiGet = async <T>(endpoint: string, token: string) =>
  unwrapEnvelope<T>(await operator.api("GET", endpoint, token));

export const operatorApiPost = async <T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
) => unwrapEnvelope<T>(await operator.api("POST", endpoint, token, body));

export const operatorApiDelete = async <T>(endpoint: string, token: string) =>
  unwrapEnvelope<T>(await operator.api("DELETE", endpoint, token));
