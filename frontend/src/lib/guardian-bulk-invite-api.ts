import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "GuardianBulkInviteAPI" });

type BulkInviteProblemReason =
  "missing_email" | "invalid_email" | "duplicate_email";

export interface BulkInviteProblem {
  guardianProfileId: string;
  guardianName: string;
  studentNames: string[];
  reason: BulkInviteProblemReason;
}

/** Counts guardians, never children: a parent of three selected children is
 *  invited once and counted once. */
export interface BulkInviteResult {
  dryRun: boolean;
  invited: number;
  linkedExistingAccount: number;
  resent: number;
  skippedActive: number;
  skippedOpen: number;
  skippedRestricted: number;
  problems: BulkInviteProblem[];
}

interface BackendBulkInviteResult {
  dry_run: boolean;
  invited: number;
  linked_existing_account: number;
  resent: number;
  skipped_active: number;
  skipped_open: number;
  skipped_restricted: number;
  problems: Array<{
    guardian_profile_id: string;
    guardian_name: string;
    student_names: string[];
    reason: BulkInviteProblemReason;
  }>;
}

interface BulkInviteResponse {
  status?: string;
  data?: BackendBulkInviteResult;
  error?: string;
}

/**
 * Invite the guardians of many children to the parents portal in one run
 * (#3378). `dryRun` only counts and mails nobody; `resendOpen` also mails
 * guardians whose invitation is still open and restarts its expiry.
 */
export async function bulkInviteGuardians(
  studentIds: string[],
  options: { dryRun: boolean; resendOpen: boolean },
): Promise<BulkInviteResult> {
  const response = await fetch("/api/guardians/bulk-invite", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      student_ids: studentIds.map((id) => Number.parseInt(id, 10)),
      dry_run: options.dryRun,
      resend_open: options.resendOpen,
    }),
  });

  if (!response.ok) {
    logger.warn("bulk_invite_guardians_failed", { status: response.status });
    throw new Error(`Failed to bulk invite guardians: ${response.status}`);
  }

  const result = (await response.json()) as BulkInviteResponse;
  if (result.status === "error" || !result.data) {
    throw new Error(result.error ?? "Failed to bulk invite guardians");
  }
  const data = result.data;
  return {
    dryRun: data.dry_run,
    invited: data.invited,
    linkedExistingAccount: data.linked_existing_account,
    resent: data.resent,
    skippedActive: data.skipped_active,
    skippedOpen: data.skipped_open,
    skippedRestricted: data.skipped_restricted,
    problems: data.problems.map((problem) => ({
      guardianProfileId: problem.guardian_profile_id,
      guardianName: problem.guardian_name,
      studentNames: problem.student_names,
      reason: problem.reason,
    })),
  };
}
