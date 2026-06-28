/**
 * The brand-blue status pill for a parent-OGS change request (offen / in
 * Bearbeitung / erledigt / …). Presentational only: the caller resolves the
 * label, since the staff portal is German-only (staffRequestStatusLabel) while
 * the parents portal is localized (parentRequestStatusI18nKey via next-intl).
 * One component so the markup/color stops being copy-pasted across the staff
 * thread page and the parent conversation. Color #5080D8 is LOCATION_COLORS.
 * OTHER_ROOM (brand blue).
 */
export function RequestStatusBadge({
  label,
  className,
}: Readonly<{ label: string; className?: string }>) {
  return (
    <span
      className={`inline-flex items-center rounded-full bg-[#5080D8]/10 px-2 py-0.5 text-xs font-medium text-[#5080D8] ${className ?? ""}`}
    >
      {label}
    </span>
  );
}
