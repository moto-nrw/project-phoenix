/**
 * Admin staff-view preview (#2893): API client + session orchestration.
 *
 * An admin picks a staff member; the backend mints a READ-ONLY access token
 * carrying that person's identity and permissions. The NextAuth session swaps
 * onto it (parking the admin's own tokens) so every page, the sidebar, and
 * every server-side read renders exactly what that person sees. Ending the
 * preview restores the parked admin state.
 */

import { signOut } from "next-auth/react";

import { apiErrorFromResponse } from "~/lib/api-error";
import { createLogger } from "~/lib/logger";
import { sessionFetch, clearSessionCache } from "~/lib/session-cache";

const logger = createLogger({ component: "StaffPreviewApi" });

export interface StaffPreviewCandidate {
  accountId: string;
  firstName: string;
  lastName: string;
  email: string;
  roles: string[];
}

interface BackendStaffPreviewCandidate {
  // int64 on the backend, serialized as a string so no ID passes through a
  // JavaScript number.
  account_id: string;
  first_name: string;
  last_name: string;
  email: string;
  roles: string[];
}

export interface StaffPreviewSession {
  expiresIn: number;
  targetAccountId: string;
  targetName: string;
}

interface BackendStaffPreviewStartResponse {
  expires_in: number;
  target_account_id: string;
  target_name: string;
}

type SessionUpdate = (data: Record<string, unknown>) => Promise<unknown>;
type SwrMutateAll = (
  matcher: (key: unknown) => boolean,
  data: undefined,
  opts: { revalidate: boolean },
) => Promise<unknown>;

/**
 * Read which staff member the session `update()` hands back is previewing.
 *
 * `null` means: the returned value does not show a running preview — either
 * because none is running, or because nothing usable came back at all. Both
 * cases are treated the same way at the start: only a session that names the
 * requested person proves the swap happened.
 */
function readPreviewTarget(result: unknown): string | null {
  if (typeof result !== "object" || result === null) return null;
  const user = (result as { user?: unknown }).user;
  if (typeof user !== "object" || user === null) return null;
  const target = (user as { previewTargetAccountId?: unknown })
    .previewTargetAccountId;
  return typeof target === "string" ? target : null;
}

/**
 * Reconcile a session that stayed in the preview after its end was recorded.
 *
 * The audit trail says the preview is over, so no page may keep rendering with
 * the target's token. Signing out is the only state both sides agree on; the
 * admin logs in again.
 */
async function signOutAfterFailedRestore(): Promise<void> {
  try {
    await signOut({ redirect: false });
  } catch (err) {
    logger.error("staff_preview_signout_failed", {
      error: err instanceof Error ? err.message : String(err),
    });
  }
  clearSessionCache();
}

/** Fetch the selectable staff members of the current school. */
export async function fetchStaffPreviewCandidates(): Promise<
  StaffPreviewCandidate[]
> {
  const response = await sessionFetch("/api/auth/staff-preview/candidates");
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      `Failed to load preview candidates: ${response.status}`,
    );
  }
  // createGetHandler wraps GET responses in { status, data }.
  const envelope = (await response.json()) as {
    data?: BackendStaffPreviewCandidate[] | null;
  };
  return (envelope.data ?? []).map((candidate) => ({
    accountId: candidate.account_id,
    firstName: candidate.first_name,
    lastName: candidate.last_name,
    email: candidate.email,
    roles: candidate.roles,
  }));
}

async function startStaffPreview(
  accountId: string,
): Promise<StaffPreviewSession> {
  const response = await sessionFetch("/api/auth/staff-preview", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ account_id: accountId }),
  });
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      `Failed to start preview: ${response.status}`,
    );
  }
  const data = (await response.json()) as BackendStaffPreviewStartResponse;
  return {
    expiresIn: data.expires_in,
    targetAccountId: data.target_account_id,
    targetName: data.target_name,
  };
}

/**
 * Report the end of a preview to the audit trail. Best effort — the preview is
 * over regardless of the answer, and the backend records one end per preview
 * instance however often it is asked.
 *
 * The signed preview token is the credential the whole way down. A running
 * preview's token lives only in the session cookie (#5005), so the route reads
 * it there. Failed starts are rolled back entirely server-side.
 * Plain fetch, not sessionFetch: sessionFetch would throw when the
 * admin session has already expired, and the call must never fail the end.
 */
export async function recordStaffPreviewEnd(
  signal?: AbortSignal,
): Promise<void> {
  signal?.throwIfAborted();
  try {
    await fetch("/api/auth/staff-preview/end", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({}),
      // The end of the preview waits for this call; a stalled request must
      // not keep the admin in the preview.
      signal: signal
        ? AbortSignal.any([signal, AbortSignal.timeout(5000)])
        : AbortSignal.timeout(5000),
    });
  } catch {
    // Audit only — ending the preview must never fail on this call.
  }
  signal?.throwIfAborted();
}

/**
 * Full start sequence: the BFF mints and installs the preview session,
 * drop every cache. Callers finish with a hard reload so all mounted data
 * re-fetches through the target's eyes.
 */
export async function performStartStaffPreview(
  accountId: string,
  swrMutate: SwrMutateAll,
): Promise<StaffPreviewSession> {
  const preview = await startStaffPreview(accountId);
  await swrMutate(() => true, undefined, { revalidate: false });
  clearSessionCache();
  return preview;
}

/**
 * Full end sequence: record the end for the audit trail (best effort), then
 * restore the admin session. The session swap can make PersonalStorageGuard
 * navigate immediately, so the audit request must settle before that swap.
 *
 * The preview token is the proof of which preview is being closed: the backend
 * reads the previewed person from the signed token, so no client can write an
 * audit entry about a preview it never held. The route takes it from the
 * session cookie, so the call must run BEFORE the session swaps back to the
 * admin, and it runs even when restoring the admin session fails: a preview
 * whose end never reaches the trail would stay open in the protocol forever.
 *
 * Fails the restore, the two sides would disagree: the trail says the preview
 * ended while the session cookie still carries the target's token. That state
 * is reconciled here — the session is signed out, so nothing keeps rendering
 * as the target after the end was recorded. The original error is rethrown so
 * the caller still sees why.
 */
export async function performEndStaffPreview(
  update: SessionUpdate,
  swrMutate: SwrMutateAll,
): Promise<void> {
  await recordStaffPreviewEnd();
  let restoreError: unknown = null;
  try {
    const result = await update({ previewEnd: true });
    if (typeof readPreviewTarget(result) === "string") {
      restoreError = new Error("preview session was not restored");
    }
  } catch (err) {
    restoreError = err;
  }
  clearSessionCache();
  if (restoreError) {
    logger.error("staff_preview_restore_failed", {
      error:
        restoreError instanceof Error
          ? restoreError.message
          : String(restoreError),
    });
    await signOutAfterFailedRestore();
    throw restoreError;
  }
  await swrMutate(() => true, undefined, { revalidate: false });
  clearSessionCache();
}
