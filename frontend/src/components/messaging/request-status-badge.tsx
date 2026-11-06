import { StatusBadge } from "~/components/ui/status-badge";

/**
 * The brand-blue status pill for a parent-OGS change request (offen / in
 * Bearbeitung / erledigt / …). Presentational only: the caller resolves the
 * label, since the staff portal is German-only (staffRequestStatusLabel) while
 * the parents portal is localized (parentRequestStatusI18nKey via next-intl).
 * One component so the markup/color stops being copy-pasted across the staff
 * thread page and the parent conversation.
 */
export function RequestStatusBadge({ label }: Readonly<{ label: string }>) {
  return <StatusBadge label={label} tone="blue" compact />;
}
