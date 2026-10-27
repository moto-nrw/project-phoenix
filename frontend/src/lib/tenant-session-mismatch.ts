import type { ErrorCode } from "./error-codes.generated";

/**
 * The BFF refused a request because the session belongs to another school
 * than this tab's host (#5130): another tab switched the shared cookie. The
 * request did not reach the backend. TenantGuard listens, drops this tab's
 * caches and switches the session back to the host's school.
 */
export const TENANT_SESSION_MISMATCH =
  "identity.tenant_session_mismatch" satisfies ErrorCode;

const listeners = new Set<() => void>();

/** Subscribes to refusals; returns the unsubscribe function. */
export function onTenantSessionMismatch(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function reportTenantSessionMismatch(): void {
  for (const listener of listeners) listener();
}

/** Whether an error envelope carries the refusal's code. */
export function isTenantSessionMismatchBody(body: unknown): boolean {
  return (
    typeof body === "object" &&
    body !== null &&
    (body as { code?: unknown }).code === TENANT_SESSION_MISMATCH
  );
}

/**
 * Reports the refusal if the response is one. Reads a clone, so the caller
 * still owns the body for its own error handling.
 */
export async function noteTenantSessionMismatch(
  response: Response,
): Promise<boolean> {
  if (response.status !== 409) return false;
  let body: unknown = null;
  try {
    body = await response.clone().json();
  } catch {
    // Not an error envelope, so not the refusal.
  }
  if (!isTenantSessionMismatchBody(body)) return false;
  reportTenantSessionMismatch();
  return true;
}
