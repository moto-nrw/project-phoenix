import type { ChildStatus } from "~/lib/enrollment-admin-api";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";

// Shared label + tone maps for enrollment child statuses. Previously duplicated
// verbatim in admin-enrollment-detail and admin-enrollment-phase-detail (#1629).
export const CHILD_STATUS_LABELS: Record<ChildStatus, string> = {
  submitted: "Eingegangen",
  under_review: "In Prüfung",
  approved: "Bestätigt",
  waitlisted: "Warteliste",
  rejected: "Abgelehnt",
  withdrawn: "Zurückgezogen",
  pending_renewal: "Wartet auf Verlängerung",
  auto_renewed: "Vorgemerkt",
  pending_admin_review: "Manuelle Prüfung",
};

const CHILD_STATUS_TONES: Record<ChildStatus, StatusBadgeTone> = {
  submitted: "blue",
  under_review: "blue",
  approved: "green",
  waitlisted: "orange",
  rejected: "red",
  withdrawn: "gray",
  pending_renewal: "orange",
  auto_renewed: "blue",
  pending_admin_review: "gray",
};

/**
 * Review reason of a renewal the automatic approval at the school-year change
 * skipped because the Kinderkontingent was full (#3570).
 */
const CHILD_QUOTA_REVIEW_REASON = "child_quota_reached";

/** True while an enrollment stays open because the Kinderkontingent is full. */
export function isHeldForChildQuota(
  child: Readonly<{ status: ChildStatus; review_reason?: string | null }>,
): boolean {
  return (
    child.review_reason === CHILD_QUOTA_REVIEW_REASON &&
    (child.status === "submitted" || child.status === "under_review")
  );
}

export function ChildQuotaHeldBadge() {
  return <StatusBadge label="Wegen Kinderkontingent offen" tone="orange" />;
}

export function ChildStatusBadge({
  status,
}: Readonly<{ status: ChildStatus }>) {
  return (
    <StatusBadge
      label={CHILD_STATUS_LABELS[status]}
      tone={CHILD_STATUS_TONES[status]}
    />
  );
}
