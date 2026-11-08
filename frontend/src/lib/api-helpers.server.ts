import { backendJson } from "./backend-client.server";

export {
  ApiResponseError,
  bffError,
  handleApiError,
  problemResponse,
} from "./api-response-error.server";

/*
 * The tenant handlers' JSON calls: projections of backendJson, the one JSON
 * path of every portal. They run inside a tenant route handler
 * (`createGetHandler`, `tenant.post`, …), which repeats a backend 401 with
 * the renewed session unless the endpoint opts out with `retry: { off }`.
 */

export async function apiGet<T>(
  endpoint: string,
  token: string,
  options?: { signal?: AbortSignal },
): Promise<T> {
  return (
    (await backendJson<T>("GET", endpoint, token, undefined, options)) ??
    ({} as T)
  );
}

/** `idempotencyKey` makes a retried create request safe. */
export async function apiPost<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
  idempotencyKey?: string,
): Promise<T> {
  return (
    (await backendJson<T>("POST", endpoint, token, body, {
      idempotencyKey,
    })) ?? ({} as T)
  );
}

export async function apiPut<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return (await backendJson<T>("PUT", endpoint, token, body)) ?? ({} as T);
}

export async function apiPatch<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T> {
  return (await backendJson<T>("PATCH", endpoint, token, body)) ?? ({} as T);
}

/** Resolves undefined for the backend's 204. */
export async function apiDelete<T, B = unknown>(
  endpoint: string,
  token: string,
  body?: B,
): Promise<T | void> {
  return backendJson<T>("DELETE", endpoint, token, body);
}
