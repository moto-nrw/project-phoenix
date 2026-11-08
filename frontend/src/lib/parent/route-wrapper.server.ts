import {
  parentAuth,
  uncachedParentAuth,
  withParentAuth,
} from "~/server/auth/parent";
import { unwrapEnvelope } from "../backend-client.server";
import { makePortalHandlers } from "../portal-handlers.server";

/** The parents portal's route handlers, on the parent session only. */
export const parent = makePortalHandlers({
  key: "parent",
  auth: () => parentAuth(),
  uncachedAuth: () => uncachedParentAuth(),
  withAuth: (handler) => withParentAuth(handler),
});

export const {
  proxyGet,
  proxyPost,
  proxyPut,
  proxyPatch,
  proxyDelete,
  del: createParentDeleteHandler,
  put: createParentPutHandler,
  patch: createParentPatchHandler,
} = parent;

export const parentApiDelete = async <T>(endpoint: string, token: string) =>
  unwrapEnvelope<T>(await parent.api("DELETE", endpoint, token));

export const parentApiPut = async <T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
) => unwrapEnvelope<T>(await parent.api("PUT", endpoint, token, body));

export const parentApiPatch = async <T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
) => unwrapEnvelope<T>(await parent.api("PATCH", endpoint, token, body));
