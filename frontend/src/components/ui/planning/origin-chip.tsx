import { StatusBadge } from "~/components/ui/status-badge";

interface OriginChipProps {
  /** Short source label, e.g. "Soll aus Arbeitszeitmodell". */
  readonly label: string;
  /** Optional long-form explanation, shown as native tooltip. */
  readonly title?: string;
}

/**
 * OriginChip names the data source of a figure ("Herkunfts-Chip"). It is
 * deliberately dosed: the planning redesign allows it at exactly two spots,
 * the InstanceDetailModal and the Soll value of the time-tracking surfaces.
 * The Betreuungsplan header names its source as plain text in the context
 * line. Do not scatter it across arbitrary figures.
 */
export function OriginChip({ label, title }: OriginChipProps) {
  return <StatusBadge tone="outline" compact label={label} title={title} />;
}
