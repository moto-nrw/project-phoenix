import { StatusBadge } from "~/components/ui/status-badge";
import { externalStaffLabel } from "~/lib/staff-helpers";

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
