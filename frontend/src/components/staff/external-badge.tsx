import { StatusBadge } from "~/components/ui/status-badge";

/**
 * Marks an external caregiver without a moto account (#3823). The
 * organization stays visible text, so a phone shows it without a tooltip.
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
      accessibleLabel="Externe Person ohne moto-Konto: "
    />
  );
}
