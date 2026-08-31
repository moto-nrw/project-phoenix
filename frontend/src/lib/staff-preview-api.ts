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
  account_id: number;
  first_name: string;
  last_name: string;
  email: string;
  roles: string[];
}

export interface StaffPreviewSession {
  accessToken: string;
  expiresIn: number;
  targetAccountId: number;
  targetName: string;
}

interface BackendStaffPreviewStartResponse {
  access_token: string;
  expires_in: number;
  target_account_id: number;
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
    accountId: candidate.account_id.toString(),
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
    body: JSON.stringify({ account_id: Number(accountId) }),
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
  await update({
    previewStart: {
      accessToken: preview.accessToken,
      expiresIn: preview.expiresIn,
      targetAccountId: preview.targetAccountId,
      targetName: preview.targetName,
    },
  });
  await swrMutate(() => true, undefined, { revalidate: false });
  clearSessionCache();
  return preview;
}

/**
 * Full end sequence: restore the admin session, then record the end for the
 * audit trail (best effort — the preview token simply expires either way).
 */
export async function performEndStaffPreview(
  targetAccountId: string | undefined,
  update: SessionUpdate,
  swrMutate: SwrMutateAll,
): Promise<void> {
  await update({ previewEnd: true });
  clearSessionCache();
  if (targetAccountId) {
    try {
      await sessionFetch("/api/auth/staff-preview/end", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ account_id: Number(targetAccountId) }),
      });
    } catch {
      // Audit only — ending the preview must never fail on this call.
    }
  }
  await swrMutate(() => true, undefined, { revalidate: false });
  clearSessionCache();
}
