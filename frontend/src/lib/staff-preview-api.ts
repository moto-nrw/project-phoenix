/**
 * Admin staff-view preview (#2893): API client + session orchestration.
 *
 * An admin picks a staff member; the backend mints a READ-ONLY access token
 * carrying that person's identity and permissions. The NextAuth session swaps
 * onto it (parking the admin's own tokens) so every page, the sidebar, and
 * every server-side read renders exactly what that person sees. Ending the
 * preview restores the parked admin state.
 */

import { sessionFetch, clearSessionCache } from "~/lib/session-cache";

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
  accessToken: string;
  expiresIn: number;
  targetAccountId: string;
  targetName: string;
}

interface BackendStaffPreviewStartResponse {
  access_token: string;
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

/** Fetch the selectable staff members of the current school. */
export async function fetchStaffPreviewCandidates(): Promise<
  StaffPreviewCandidate[]
> {
  const response = await sessionFetch("/api/auth/staff-preview/candidates");
  if (!response.ok) {
    throw new Error(`Failed to load preview candidates: ${response.status}`);
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
    throw new Error(`Failed to start preview: ${response.status}`);
  }
  const data = (await response.json()) as BackendStaffPreviewStartResponse;
  return {
    accessToken: data.access_token,
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
 * Plain fetch, not sessionFetch: the signed preview token in the body is the
 * credential the whole way down, and the route needs no session. sessionFetch
 * would throw when the admin session has already expired — exactly the case in
 * which the end must still reach the audit trail.
 */
async function postPreviewEnd(previewToken: string): Promise<void> {
  try {
    await fetch("/api/auth/staff-preview/end", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ preview_token: previewToken }),
    });
  } catch {
    // Audit only — ending the preview must never fail on this call.
  }
}

/**
 * Full start sequence: mint the preview token, swap the session onto it,
 * drop every cache. Callers finish with a hard reload so all mounted data
 * re-fetches through the target's eyes.
 */
export async function performStartStaffPreview(
  accountId: string,
  update: SessionUpdate,
  swrMutate: SwrMutateAll,
): Promise<StaffPreviewSession> {
  const preview = await startStaffPreview(accountId);
  try {
    await update({
      previewStart: {
        accessToken: preview.accessToken,
        expiresIn: preview.expiresIn,
        targetAccountId: preview.targetAccountId,
        targetName: preview.targetName,
      },
    });
  } catch (err) {
    // The backend already minted the token and recorded "preview started".
    // The session never swapped, so no preview exists in the UI — close it in
    // the audit trail too, otherwise the start stays unmatched forever.
    await postPreviewEnd(preview.accessToken);
    throw err;
  }
  await swrMutate(() => true, undefined, { revalidate: false });
  clearSessionCache();
  return preview;
}

/**
 * Full end sequence: restore the admin session, then record the end for the
 * audit trail (best effort — the preview token simply expires either way).
 *
 * The preview token travels with that call as proof of which preview is being
 * closed: the backend reads the previewed person from the signed token, so no
 * client can write an audit entry about a preview it never held. It is
 * therefore captured BEFORE the session swaps back to the admin.
 */
export async function performEndStaffPreview(
  previewToken: string | undefined,
  update: SessionUpdate,
  swrMutate: SwrMutateAll,
): Promise<void> {
  await update({ previewEnd: true });
  clearSessionCache();
  if (previewToken) {
    await postPreviewEnd(previewToken);
  }
  await swrMutate(() => true, undefined, { revalidate: false });
  clearSessionCache();
}
