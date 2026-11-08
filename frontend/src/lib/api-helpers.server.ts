import {
  readBackendJson,
  sendBackend,
  type BackendMethod,
} from "./backend-client.server";

export {
  ApiResponseError,
  bffError,
  handleApiError,
  problemResponse,
} from "./api-response-error.server";

/**
 * The tenant backend's JSON, envelope included, for a token the route
 * handler holds. A backend error throws ApiResponseError; the tenant route
 * handlers repeat a 401 with the renewed session.
 */
async function tenantJson<T>(
  method: BackendMethod,
  endpoint: string,
  token: string,
  init: { body?: unknown; idempotencyKey?: string; signal?: AbortSignal } = {},
): Promise<T> {
  const response = await sendBackend(endpoint, token, {
    method,
    json: init.body ?? undefined,
    idempotencyKey: init.idempotencyKey,
    signal: init.signal,
  });
  return readBackendJson<T>(response);
}

export async function apiGet<T>(
  endpoint: string,
  token: string,
  options?: { signal?: AbortSignal },
): Promise<T> {
  return (await tenantJson<T>("GET", endpoint, token, options)) ?? ({} as T);
}

/** `idempotencyKey` makes a retried create request safe. */
export async function apiPost<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
  idempotencyKey?: string,
): Promise<T> {
  return (
    (await tenantJson<T>("POST", endpoint, token, { body, idempotencyKey })) ??
    ({} as T)
  );
}

export async function apiPut<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return (await tenantJson<T>("PUT", endpoint, token, { body })) ?? ({} as T);
}

export async function apiPatch<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return (await tenantJson<T>("PATCH", endpoint, token, { body })) ?? ({} as T);
}

/** Resolves undefined for the backend's 204. */
export async function apiDelete<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T | void> {
  return tenantJson<T>("DELETE", endpoint, token, { body });
}
