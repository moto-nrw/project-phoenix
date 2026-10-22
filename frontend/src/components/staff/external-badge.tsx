import { StatusBadge } from "~/components/ui/status-badge";
import type { ExternalAccountStatus } from "~/lib/staff-api";
import { externalAccountMark, externalStaffLabel } from "~/lib/staff-helpers";

/**
 * Marks an external caregiver (#3823), with or without a moto account
 * (#4573). The organization stays visible text, so a phone shows it without
 * a tooltip.
 */
export function ExternalBadge({
  organization,
}: {
  readonly organization?: string | null;
}) {
  return (
    <StatusBadge
      tone="gray"
      compact
      label={externalStaffLabel(organization)}
      accessibleLabel="Externe Person: "
    />
  );
}

/**
 * Account state of an external caregiver (#4573): „Einladung offen“ while the
 * invitation waits, „Kein Konto“ without one. Nothing for an account.
 */
export function ExternalAccountBadge({
  status,
}: {
  readonly status?: ExternalAccountStatus | null;
}) {
  const mark = externalAccountMark(status);
  if (!mark) return null;
  return <StatusBadge tone={mark.tone} compact label={mark.label} />;
}
