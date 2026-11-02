import type { OverflowMenuItem } from "~/components/ui/page-header/types";

interface BuildArgs {
  /** Whether the current group is being supervised via substitution. */
  readonly viaSubstitution: boolean;
  /** Whether the caller has a fixed responsibility for this group. */
  readonly canTransfer: boolean;
  /** Number of currently outgoing transfers — surfaced as a badge. */
  readonly activeTransfersCount: number;
  /** Opens the group transfer modal. */
  readonly onOpenTransfer: () => void;
  /**
   * Opens the weekly protocols of the group (#4769). Given only when the
   * caller sees them: an own or substituted group, or the leadership.
   */
  readonly onOpenWeekProtocols?: () => void;
}

/**
 * Builds the kebab-menu items for the OGS-Groups page header. Returns an
 * empty array when nothing is appropriate (e.g. while supervising via
 * substitution — you can't transfer a group you don't own), which causes
 * `<OverflowMenu>` to render nothing.
 */
export function buildGroupOverflowItems({
  viaSubstitution,
  canTransfer,
  activeTransfersCount,
  onOpenTransfer,
  onOpenWeekProtocols,
}: BuildArgs): OverflowMenuItem[] {
  const items: OverflowMenuItem[] = [];
  if (!viaSubstitution && canTransfer) {
    items.push({
      label: "Gruppe übergeben",
      badge: activeTransfersCount > 0 ? activeTransfersCount : undefined,
      onClick: onOpenTransfer,
    });
  }
  if (onOpenWeekProtocols) {
    items.push({ label: "Wochenprotokolle", onClick: onOpenWeekProtocols });
  }
  return items;
}
