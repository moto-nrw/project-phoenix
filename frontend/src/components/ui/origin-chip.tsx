import type { ReactNode } from "react";

interface OriginChipProps {
  /** Short source label, e.g. "Soll aus Arbeitszeitmodell". */
  readonly label: string;
  /** Optional long-form explanation, shown as native tooltip. */
  readonly title?: string;
  /** Optional 12px leading icon. */
  readonly icon?: ReactNode;
  readonly className?: string;
}

/**
 * OriginChip names the data source of a figure ("Herkunfts-Chip"). It is
 * deliberately dosed: the planning redesign allows it at exactly three
 * spots — the Betreuungsplan header, the InstanceDetailModal, and the
 * Soll value of the time-tracking surfaces (docs/planung-redesign/docs/
 * 04-designsprache.md section 6.2). Do not scatter it across arbitrary
 * figures.
 */
export function OriginChip({ label, title, icon, className }: OriginChipProps) {
  return (
    <span
      title={title}
      className={`inline-flex items-center gap-1 rounded-full border border-gray-200 bg-gray-50 px-2 py-0.5 text-[11px] text-gray-600 ${className ?? ""}`}
    >
      {icon}
      {label}
    </span>
  );
}
