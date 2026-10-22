import { StatusBadge } from "~/components/ui/status-badge";
import type { ExternalAccountStatus } from "~/lib/staff-api";
import { externalAccountLabel } from "~/lib/staff-helpers";

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
  const trimmed = organization?.trim();
  return (
    <StatusBadge
      tone="gray"
      compact
      label={trimmed ? `Extern · ${trimmed}` : "Extern"}
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
  const label = externalAccountLabel(status);
  if (!label) return null;
  return (
    <StatusBadge
      tone={status === "invitation_pending" ? "orange" : "gray"}
      compact
      label={label}
    />
  );
}
