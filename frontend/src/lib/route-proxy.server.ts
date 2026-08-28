import * as api from "./api-helpers.server";
import { makeProxyFactories } from "./route-proxy-factory.server";
import {
  createDeleteHandler,
  createGetHandler,
  createPatchHandler,
  createPostHandler,
  createPutHandler,
} from "./route-wrapper.server";

/**
 * Tenant proxy factories — `proxyGet`/`proxyPost`/`proxyPut`/`proxyPatch`/
 * `proxyDelete` for pure pass-through routes, instead of hand-rolling
 * `createXHandler(async … apiX(…))`. The tenant fetchers return the raw backend
 * envelope, so these unwrap `data` by default; pass `{ raw: true }` to forward
 * the whole response (e.g. paginated payloads).
 *
 * This lives in its own module (not `route-wrapper.server.ts`) and reaches the
 * API fetchers through a namespace import + lazy generic wrappers so that the
 * module never reads individual `api-helpers.server` exports at evaluation
 * time. `route-wrapper.server.ts` is imported by nearly every route test, which
 * mock `api-helpers.server` partially; eagerly binding extra named exports
 * there would make those mocks throw on load.
 */
export const { proxyGet, proxyPost, proxyPut, proxyPatch, proxyDelete } =
  makeProxyFactories({
    get: createGetHandler,
    post: createPostHandler,
    put: createPutHandler,
    patch: createPatchHandler,
    del: createDeleteHandler,
    apiGet: <T>(endpoint: string, token: string) =>
      api.apiGet<T>(endpoint, token),
    apiPost: <T, B = unknown>(
      endpoint: string,
      token: string,
      body?: B,
      idempotencyKey?: string,
    ) =>
      idempotencyKey
        ? api.apiPost<T, B>(endpoint, token, body, idempotencyKey)
        : api.apiPost<T, B>(endpoint, token, body),
    apiPut: <T, B = unknown>(endpoint: string, token: string, body?: B) =>
      api.apiPut<T, B>(endpoint, token, body),
    apiPatch: <T, B = unknown>(endpoint: string, token: string, body?: B) =>
      api.apiPatch<T, B>(endpoint, token, body),
    apiDelete: (endpoint: string, token: string) =>
      api.apiDelete(endpoint, token),
    fetcherUnwrapsData: false,
  });
